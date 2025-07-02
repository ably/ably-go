package objects

// Plugin is used to add LiveObject functionality to a realtime client:
//
//	client, err := ably.NewRealtime(
//		ably.WithKey(...),
//		ably.WithExperimentalObjectsPlugin(myObjectsPlugin),
//	)
//
// The plugin is used to prepare object messages to be published in the
// realtime channel's publishObjects method, and is used to handle incoming
// object and object sync messages.
type Plugin interface {
	// PrepareObject prepares an object to be published (e.g. setting the
	// objectId and intialValue for a COUNTER_CREATE op).
	PrepareObject(msg *Message) error

	// HandleObjectMessages is called to handle incoming object messages.
	HandleObjectMessages(msgs []*Message)

	// HandleObjectSyncMessages is called to handle incoming object sync
	// messages.
	HandleObjectSyncMessages(msgs []*Message, channelSerial string)
}

type Message struct {
	ID           string                 `json:"id,omitempty" codec:"id,omitempty"`
	Timestamp    int64                  `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	ClientID     string                 `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionID string                 `json:"connectionId,omitempty" codec:"connectionId,omitempty"`
	Operation    *Operation             `json:"operation,omitempty" codec:"operation,omitempty"`
	Object       *State                 `json:"object,omitempty" codec:"object,omitempty"`
	Extras       map[string]interface{} `json:"extras,omitempty" codec:"extras,omitempty"`
	Serial       string                 `json:"serial,omitempty" codec:"serial,omitempty"`
	SiteCode     string                 `json:"siteCode,omitempty" codec:"siteCode,omitempty"`
}

type State struct {
	ObjectId        string            `json:"objectId,omitempty" codec:"objectId,omitempty"`
	SiteTimeserials map[string]string `json:"siteTimeserials,omitempty" codec:"siteTimeserials,omitempty"`
	Tombstone       bool              `json:"tombstone,omitempty" codec:"tombstone,omitempty"`
	CreateOp        *Operation        `json:"createOp,omitempty" codec:"createOp,omitempty"`
	Map             *Map              `json:"map,omitempty" codec:"map,omitempty"`
	Counter         *Counter          `json:"counter,omitempty" codec:"counter,omitempty"`
}

type OperationAction int

const (
	Operation_MapCreate     OperationAction = 0
	Operation_MapSet        OperationAction = 1
	Operation_MapRemove     OperationAction = 2
	Operation_CounterCreate OperationAction = 3
	Operation_CounterInc    OperationAction = 4
	Operation_Delete        OperationAction = 5
)

func (o OperationAction) String() string {
	switch o {
	case Operation_MapCreate:
		return "MAP_CREATE"
	case Operation_MapSet:
		return "MAP_SET"
	case Operation_MapRemove:
		return "MAP_REMOVE"
	case Operation_CounterCreate:
		return "COUNTER_CREATE"
	case Operation_CounterInc:
		return "COUNTER_INC"
	case Operation_Delete:
		return "OBJECT_DELETE"
	default:
		return ""
	}
}

type Operation struct {
	Action       OperationAction `json:"action,omitempty" codec:"action,omitempty"`
	ObjectID     string          `json:"objectId,omitempty" codec:"objectId,omitempty"`
	MapOp        *MapOp          `json:"mapOp,omitempty" codec:"mapOp,omitempty"`
	CounterOp    *CounterOp      `json:"counterOp,omitempty" codec:"counterOp,omitempty"`
	Map          *Map            `json:"map,omitempty" codec:"map,omitempty"`
	Counter      *Counter        `json:"counter,omitempty" codec:"counter,omitempty"`
	Nonce        *string         `json:"nonce,omitempty" codec:"nonce,omitempty"`
	InitialValue string          `json:"initialValue,omitempty" codec:"initialValue,omitempty"`
}

type CounterOp struct {
	Amount float64 `json:"amount,omitempty" codec:"amount,omitempty"`
}

type Counter struct {
	Count float64 `json:"count,omitempty" codec:"count,omitempty"`
}

type MapOp struct {
	Key  string `json:"key,omitempty" codec:"key,omitempty"`
	Data *Data  `json:"data,omitempty" codec:"data,omitempty"`
}

type Data struct {
	ObjectId *string  `json:"objectId,omitempty" codec:"objectId,omitempty"`
	Encoding *string  `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Boolean  *bool    `json:"boolean,omitempty" codec:"boolean,omitempty"`
	Bytes    []byte   `json:"bytes,omitempty" codec:"bytes,omitempty"`
	Number   *float64 `json:"number,omitempty" codec:"number,omitempty"`
	String   *string  `json:"string,omitempty" codec:"string,omitempty"`
}

type Map struct {
	Semantics MapSemantics         `json:"semantics,omitempty" codec:"semantics,omitempty"`
	Entries   map[string]*MapEntry `json:"entries,omitempty" codec:"entries,omitempty"`
}

type MapSemantics int

const (
	Map_LWW MapSemantics = 0
)

func (m MapSemantics) String() string {
	switch m {
	case Map_LWW:
		return "LWW"
	default:
		return ""
	}
}

type MapEntry struct {
	Tombstone  bool    `json:"tombstone,omitempty" codec:"tombstone,omitempty"`
	Timeserial *string `json:"timeserial,omitempty" codec:"timeserial,omitempty"`
	Data       *Data   `json:"data,omitempty" codec:"data,omitempty"`
}
