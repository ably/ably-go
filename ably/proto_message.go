package ably

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// encodings
const (
	encUTF8   = "utf-8"
	encJSON   = "json"
	encBase64 = "base64"
	encCipher = "cipher"
	encVCDiff = "vcdiff"
)

// Message contains an individual message that is sent to, or received from, Ably.
type Message struct {
	// ID is a unique identifier assigned by Ably to this message (TM2a).
	ID string `json:"id,omitempty" codec:"id,omitempty"`
	// ClientID is of the publisher of this message (RSL1g1, TM2b).
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`
	// ConnectionID of the publisher of this message (TM2c).
	ConnectionID string `json:"connectionId,omitempty" codec:"connectionId,omitempty"`
	// Deprecated: This attribute is deprecated and will be removed in future versions
	// ConnectionKey is a connectionKey of the active connection.
	ConnectionKey string `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	// Name is the event name (TM2g).
	Name string `json:"name,omitempty" codec:"name,omitempty"`
	// Data is the message payload, if provided (TM2d).
	Data interface{} `json:"data,omitempty" codec:"data,omitempty"`
	// Encoding is typically empty, as all messages received from Ably are automatically decoded client-side
	// using this value. However, if the message encoding cannot be processed, this attribute contains the remaining
	// transformations not applied to the data payload (TM2e).
	Encoding string `json:"encoding,omitempty" codec:"encoding,omitempty"`
	// Timestamp of when the message was received by Ably, as milliseconds since the Unix epoch (TM2f).
	Timestamp int64 `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	// Extras is a JSON object of arbitrary key-value pairs that may contain metadata, and/or ancillary payloads.
	// Valid payloads include push, deltaExtras, ReferenceExtras and headers (TM2i).
	Extras map[string]interface{} `json:"extras,omitempty" codec:"extras,omitempty"`
}

// DeltaExtras describes a message whose payload is a "vcdiff"-encoded delta generated with respect to a base message (DE1, DE2).
type DeltaExtras struct {
	// From is the ID of the base message the delta was generated from (DE2a).
	From string `json:"from,omitempty"`
	// Format is the delta format; currently only "vcdiff" is allowed (DE2b).
	Format string `json:"format,omitempty"`
}

// extractDeltaExtras extracts delta information from the message extras field.
// Returns nil if no delta information is present.
func extractDeltaExtras(extras map[string]interface{}) *DeltaExtras {
	if extras == nil {
		return nil
	}

	deltaData, ok := extras["delta"]
	if !ok {
		return nil
	}

	// Try to parse as map[string]interface{}
	deltaMap, ok := deltaData.(map[string]interface{})
	if !ok {
		return nil
	}

	var deltaExtras DeltaExtras
	if from, ok := deltaMap["from"].(string); ok {
		deltaExtras.From = from
	}
	if format, ok := deltaMap["format"].(string); ok {
		deltaExtras.Format = format
	}

	return &deltaExtras
}

// DecodingContext provides context needed for decoding messages, including delta support.
type DecodingContext struct {
	// VCDiffPlugin is the plugin to use for vcdiff delta decoding (PC3).
	VCDiffPlugin VCDiffDecoder
	// BasePayload is the stored base payload for delta decoding (RTL19).
	BasePayload []byte
	// LastMessageID is the ID of the last message used for delta validation (RTL20).
	LastMessageID string
}

func (p *protocolMessage) updateInnerMessageEmptyFields(m *Message, index int) {
	if empty(m.ID) {
		m.ID = fmt.Sprintf("%s:%d", p.ID, index)
	}
	if empty(m.ConnectionID) {
		m.ConnectionID = p.ConnectionID
	}
	if m.Timestamp == 0 {
		m.Timestamp = p.Timestamp
	}
}

// updateInnerMessagesEmptyFields updates [Message.ID], [Message.ConnectionID] and [Message.Timestamp] with
// outer/parent message fields.
func (p *protocolMessage) updateInnerMessagesEmptyFields() {
	for i, m := range p.Messages {
		p.updateInnerMessageEmptyFields(m, i)
	}
	for i, m := range p.Presence {
		p.updateInnerMessageEmptyFields(&m.Message, i)
	}
}

func (m Message) String() string {
	return fmt.Sprintf("<Message %q data=%v>", m.Name, m.Data)
}

func unencodableDataErr(data interface{}) error {
	return fmt.Errorf("message data type %T must be string, []byte, or a value that can be encoded as a JSON object or array", data)
}

// withEncodedData - Used to encode string, binary([]byte) or json data (TM3).
// Updates/Mutates Message.Data and Message.Encoding.
func (m Message) withEncodedData(cipher channelCipher) (Message, error) {
	if m.Data == nil {
		return m, nil
	}

	// If string isn't UTF-8, convert to []byte to encode it as base64
	// below.
	if d, ok := m.Data.(string); ok && !utf8.ValidString(d) {
		m.Data = []byte(d)
	}

	switch d := m.Data.(type) {
	case string:
	case []byte:
		m.Data = base64.StdEncoding.EncodeToString(d)
		m.Encoding = mergeEncoding(m.Encoding, encBase64)
	default:
		// RSL4c3, RSL4d3: JSON is only for objects and arrays. So marshal data
		// into JSON, then check if it's one of those.
		b, err := json.Marshal(d)
		if err != nil {
			return Message{}, fmt.Errorf("%s; encoding as JSON: %w", unencodableDataErr(d), err)
		}
		token, _ := json.NewDecoder(bytes.NewReader(b)).Token()
		if token != json.Delim('[') && token != json.Delim('{') {
			return Message{}, fmt.Errorf("%s; encoded as JSON %T", unencodableDataErr(d), token)
		}
		m.Data = string(b)
		m.Encoding = mergeEncoding(m.Encoding, encJSON)
	}

	if cipher == nil {
		return m, nil
	}

	// since we know that m.Data is either []byte or string at this point, coerceBytes is always
	// safe here
	bs, err := coerceBytes(m.Data)
	if err != nil {
		panic(err)
	}
	e, err := cipher.Encrypt(bs)
	if err != nil {
		return Message{}, fmt.Errorf("encrypting message data: %w", err)
	}
	// if the plain text is a valid utf-8 string, then add utf-8 to the list
	// of encodings to indicate to the client-side decoder that the plain text
	// should be further decoded into a utf-8 string after being decrypted.
	if s, ok := m.Data.(string); ok && utf8.ValidString(s) {
		m.Encoding = mergeEncoding(m.Encoding, encUTF8)
	}
	m.Data = base64.StdEncoding.EncodeToString(e)
	m.Encoding = mergeEncoding(m.Encoding, cipher.GetAlgorithm())
	m.Encoding = mergeEncoding(m.Encoding, encBase64)
	return m, nil
}

// withDecodedData - Used to decode received encoded data into string, binary([]byte) or json (TM3).
func (m Message) withDecodedData(cipher channelCipher) (Message, error) {
	// strings.Split on empty string returns []string{""}
	if m.Data == nil || m.Encoding == "" {
		return m, nil
	}
	encodings := strings.Split(m.Encoding, "/")
	for len(encodings) > 0 {
		encoding := encodings[len(encodings)-1]
		encodings = encodings[:len(encodings)-1]
		switch encoding {
		case encBase64:
			d, err := coerceString(m.Data)
			if err != nil {
				return m, err
			}
			data, err := base64.StdEncoding.DecodeString(d)
			if err != nil {
				return m, err
			}
			m.Data = data
		case encUTF8:
			d, err := coerceString(m.Data)
			if err != nil {
				return m, err
			}
			m.Data = d
		case encJSON:
			d, err := coerceBytes(m.Data)
			if err != nil {
				return m, err
			}
			var result interface{}
			if err := json.Unmarshal(d, &result); err != nil {
				return m, fmt.Errorf("error unmarshaling JSON payload of type %T: %s", m.Data, err.Error())
			}
			m.Data = result
		default:
			if strings.HasPrefix(encoding, encCipher) {
				if cipher == nil {
					return m, fmt.Errorf("message data is encrypted as %s, but cipher wasn't provided", encoding)
				}
				d, err := coerceBytes(m.Data)
				if err != nil {
					return m, err
				}
				d, err = cipher.Decrypt(d)
				if err != nil {
					return m, fmt.Errorf("decrypting message data: %w", err)
				}
				m.Data = d
			} else {
				return m, fmt.Errorf("unknown encoding %s", encoding)
			}
		}
		m.Encoding = strings.Join(encodings, "/")
	}
	return m, nil
}

// withDecodedDataAndContext decodes message data with support for delta decoding (RTL19).
func (m Message) withDecodedDataAndContext(cipher channelCipher, ctx *DecodingContext) (Message, []byte, error) {
	// strings.Split on empty string returns []string{""}
	if m.Data == nil || m.Encoding == "" {
		return m, nil, nil
	}

	var basePayloadForReturn []byte
	encodings := strings.Split(m.Encoding, "/")

	for len(encodings) > 0 {
		encoding := encodings[len(encodings)-1]
		encodings = encodings[:len(encodings)-1]
		switch encoding {
		case encVCDiff:
			// Handle vcdiff delta decoding (PC3, RTL18, RTL19, RTL20)
			if ctx == nil || ctx.VCDiffPlugin == nil {
				return m, nil, fmt.Errorf("missing VCdiff decoder plugin (code 40019)")
			}

			// Validate delta reference ID (RTL20)
			deltaExtras := extractDeltaExtras(m.Extras)
			if deltaExtras != nil && deltaExtras.From != ctx.LastMessageID {
				return m, nil, fmt.Errorf("delta message decode failure - delta reference ID %q does not match stored ID %q (code 40018)", deltaExtras.From, ctx.LastMessageID)
			}

			if ctx.BasePayload == nil {
				return m, nil, fmt.Errorf("delta message decode failure - no base payload available (code 40018)")
			}

			deltaBytes, err := coerceBytes(m.Data)
			if err != nil {
				return m, nil, err
			}

			// Decode the delta
			result, err := ctx.VCDiffPlugin.Decode(deltaBytes, ctx.BasePayload)
			if err != nil {
				return m, nil, fmt.Errorf("vcdiff decode failed: %w (code 40018)", err)
			}

			m.Data = result
			basePayloadForReturn = result // Store as new base payload (RTL19c)

		case encBase64:
			d, err := coerceString(m.Data)
			if err != nil {
				return m, nil, err
			}
			data, err := base64.StdEncoding.DecodeString(d)
			if err != nil {
				return m, nil, err
			}
			m.Data = data
			// For non-delta messages, store as base payload after base64 decoding (RTL19a)
			if basePayloadForReturn == nil {
				basePayloadForReturn = data
			}

		case encUTF8:
			d, err := coerceString(m.Data)
			if err != nil {
				return m, nil, err
			}
			m.Data = d

		case encJSON:
			d, err := coerceBytes(m.Data)
			if err != nil {
				return m, nil, err
			}
			var result interface{}
			if err := json.Unmarshal(d, &result); err != nil {
				return m, nil, fmt.Errorf("error unmarshaling JSON payload of type %T: %s", m.Data, err.Error())
			}
			m.Data = result

		default:
			if strings.HasPrefix(encoding, encCipher) {
				if cipher == nil {
					return m, nil, fmt.Errorf("message data is encrypted as %s, but cipher wasn't provided", encoding)
				}
				d, err := coerceBytes(m.Data)
				if err != nil {
					return m, nil, err
				}
				d, err = cipher.Decrypt(d)
				if err != nil {
					return m, nil, fmt.Errorf("decrypting message data: %w", err)
				}
				m.Data = d
			} else {
				return m, nil, fmt.Errorf("unknown encoding %s", encoding)
			}
		}
		m.Encoding = strings.Join(encodings, "/")
	}

	// For non-delta messages, store the final decoded data as base payload (RTL19b)
	if basePayloadForReturn == nil {
		if data, err := coerceBytes(m.Data); err == nil {
			basePayloadForReturn = data
		} else if s, err := coerceString(m.Data); err == nil {
			basePayloadForReturn = []byte(s) // Convert string to UTF-8 bytes
		}
	}

	return m, basePayloadForReturn, nil
}

func coerceString(i interface{}) (string, error) {
	switch v := i.(type) {
	case []byte:
		return string(v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("UTF8 encoding can only handle types string or []byte, but got %T", i)
	}
}

func coerceBytes(i interface{}) ([]byte, error) {
	switch v := i.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("coerceBytes should never get anything but strings or []byte, got %T", i)
	}
}

func mergeEncoding(a string, b string) string {
	if a == "" {
		return b
	}
	return a + "/" + b
}
