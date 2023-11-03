// Code generated by errors-const v0.3.0 tool DO NOT EDIT.

package ably

// ErrorCode is the type for predefined Ably error codes.
type ErrorCode int

func (c ErrorCode) String() string {
	switch c {
	case 0:
		return "(error code not set)"
	case 10000:
		return "no error"
 	case 20000:
		return "general error code"
	case 40000:
		return "bad request"
	case 40001:
		return "invalid request body"
	case 40002:
		return "invalid parameter name"
	case 40003:
		return "invalid parameter value"
	case 40004:
		return "invalid header"
	case 40005:
		return "invalid credential"
	case 40006:
		return "invalid connection id"
	case 40007:
		return "invalid message id"
	case 40008:
		return "invalid content length"
	case 40009:
		return "maximum message length exceeded"
	case 40010:
		return "invalid channel name"
	case 40011:
		return "stale ring state"
	case 40012:
		return "invalid client id"
	case 40013:
		return "Invalid message data or encoding"
	case 40014:
		return "Resource disposed"
	case 40015:
		return "Invalid device id"
	case 40016:
		return "Invalid message name"
	case 40017:
		return "Unsupported protocol version"
	case 40018:
		return "Unable to decode message; channel attachment no longer viable"
	case 40019:
		return "Required client library plugin not present"
	case 40020:
		return "Batch error"
	case 40021:
		return "Feature requires a newer platform version"
	case 40030:
		return "Invalid publish request (unspecified)"
	case 40031:
		return "Invalid publish request (invalid client-specified id)"
	case 40032:
		return "Invalid publish request (impermissible extras field)"
	case 40099:
		return "Reserved for artifical errors for testing"
	case 40100:
		return "unauthorized"
	case 40101:
		return "invalid credentials"
	case 40102:
		return "incompatible credentials"
	case 40103:
		return "invalid use of Basic auth over non-TLS transport"
	case 40104:
		return "timestamp not current"
	case 40105:
		return "nonce value replayed"
	case 40106:
		return "Unable to obtain credentials from given parameters"
	case 40110:
		return "account disabled"
	case 40111:
		return "account restricted (connection limits exceeded)"
	case 40112:
		return "account blocked (message limits exceeded)"
	case 40113:
		return "account blocked"
	case 40114:
		return "account restricted (channel limits exceeded)"
	case 40115:
		return "maximum number of permitted applications exceeded"
	case 40120:
		return "application disabled"
	case 40121:
		return "token revocation not enabled for this application"
	case 40125:
		return "maximum number of rules per application exceeded"
	case 40126:
		return "maximum number of namespaces per application exceeded"
	case 40127:
		return "maximum number of keys per application exceeded"
	case 40130:
		return "key error (unspecified)"
	case 40131:
		return "key revoked"
	case 40132:
		return "key expired"
	case 40133:
		return "wrong key; cannot revoke tokens with a different key to the one that issued them"
	case 40140:
		return "token error (unspecified)"
	case 40141:
		return "token revoked"
	case 40142:
		return "token expired"
	case 40143:
		return "token unrecognised"
	case 40144:
		return "invalid JWT format"
	case 40145:
		return "invalid token format"
	case 40150:
		return "connection blocked (limits exceeded)"
	case 40160:
		return "operation not permitted with provided capability"
	case 40161:
		return "operation not permitted as it requires an identified client"
	case 40162:
		return "operation not permitted with a token, requires basic auth"
	case 40163:
		return "operation not permitted, key not marked as permitting revocable tokens"
	case 40170:
		return "error from client token callback"
	case 40171:
		return "no means provided to renew auth token"
	case 40300:
		return "forbidden"
	case 40310:
		return "account does not permit tls connection"
	case 40311:
		return "operation requires tls connection"
	case 40320:
		return "application requires authentication"
	case 40330:
		return "unable to activate account due to placement constraint (unspecified)"
	case 40331:
		return "unable to activate account due to placement constraint (incompatible environment)"
	case 40332:
		return "unable to activate account due to placement constraint (incompatible site)"
	case 40400:
		return "not found"
	case 40500:
		return "method not allowed"
	case 41001:
		return "push device registration expired"
	case 42200:
		return "Unprocessable entity"
	case 42910:
		return "rate limit exceeded (nonfatal): request rejected (unspecified)"
	case 42911:
		return "max per-connection publish rate limit exceeded (nonfatal): unable to publish message"
	case 42912:
		return "there is a channel iteration call already in progress; only 1 active call is permitted to execute at any one time"
	case 42920:
		return "rate limit exceeded (fatal)"
	case 42921:
		return "max per-connection publish rate limit exceeded (fatal); closing connection"
	case 50000:
		return "internal error"
	case 50001:
		return "internal channel error"
	case 50002:
		return "internal connection error"
	case 50003:
		return "timeout error"
	case 50004:
		return "Request failed due to overloaded instance"
	case 50005:
		return "Service unavailable (service temporarily in lockdown)"
	case 50010:
		return "Ably's edge proxy service has encountered an unknown internal error whilst processing the request"
	case 50210:
		return "Ably's edge proxy service received an invalid (bad gateway) response from the Ably platform"
	case 50310:
		return "Ably's edge proxy service received a service unavailable response code from the Ably platform"
	case 50320:
		return "Active Traffic Management: traffic for this cluster is being temporarily redirected to a backup service"
	case 50330:
		return "request reached the wrong cluster; retry (used during dns changes for cluster migrations)"
	case 50410:
		return "Ably's edge proxy service timed out waiting for the Ably platform"
	case 70000:
		return "reactor operation failed"
	case 70001:
		return "reactor operation failed (post operation failed)"
	case 70002:
		return "reactor operation failed (post operation returned unexpected code)"
	case 70003:
		return "reactor operation failed (maximum number of concurrent in-flight requests exceeded)"
	case 70004:
		return "reactor operation failed (invalid or unaccepted message contents)"
	case 71000:
		return "Exchange error (unspecified)"
	case 71001:
		return "Forced re-attachment due to permissions change"
	case 71100:
		return "Exchange publisher error (unspecified)"
	case 71101:
		return "No such publisher"
	case 71102:
		return "Publisher not enabled as an exchange publisher"
	case 71200:
		return "Exchange product error (unspecified)"
	case 71201:
		return "No such product"
	case 71202:
		return "Product disabled"
	case 71203:
		return "No such channel in this product"
	case 71204:
		return "Forced re-attachment due to product being remapped to a different namespace"
	case 71300:
		return "Exchange subscription error (unspecified)"
	case 71301:
		return "Subscription disabled"
	case 71302:
		return "Requester has no subscription to this product"
	case 71303:
		return "Channel does not match the channel filter specified in the subscription to this product"
	case 80000:
		return "connection failed"
	case 80001:
		return "connection failed (no compatible transport)"
	case 80002:
		return "connection suspended"
	case 80003:
		return "disconnected"
	case 80004:
		return "already connected"
	case 80005:
		return "invalid connection id (remote not found)"
	case 80006:
		return "unable to recover connection (messages expired)"
	case 80007:
		return "unable to recover connection (message limit exceeded)"
	case 80008:
		return "unable to recover connection (connection expired)"
	case 80009:
		return "connection not established (no transport handle)"
	case 80010:
		return "invalid operation (invalid transport handle)"
	case 80011:
		return "unable to recover connection (incompatible auth params)"
	case 80012:
		return "unable to recover connection (invalid or unspecified connection serial)"
	case 80013:
		return "protocol error"
	case 80014:
		return "connection timed out"
	case 80015:
		return "incompatible connection parameters"
	case 80016:
		return "operation on superseded connection"
	case 80017:
		return "connection closed"
	case 80018:
		return "invalid connection id (invalid format)"
	case 80019:
		return "client configured authentication provider request failed"
	case 80020:
		return "continuity loss due to maximum subscribe message rate exceeded"
	case 80021:
		return "exceeded maximum permitted account-wide rate of creating new connections"
	case 80030:
		return "client restriction not satisfied"
	case 90000:
		return "channel operation failed"
	case 90001:
		return "channel operation failed (invalid channel state)"
	case 90002:
		return "channel operation failed (epoch expired or never existed)"
	case 90003:
		return "unable to recover channel (messages expired)"
	case 90004:
		return "unable to recover channel (message limit exceeded)"
	case 90005:
		return "unable to recover channel (no matching epoch)"
	case 90006:
		return "unable to recover channel (unbounded request)"
	case 90007:
		return "channel operation failed (no response from server)"
	case 90010:
		return "maximum number of channels per connection/request exceeded"
	case 90021:
		return "exceeded maximum permitted account-wide rate of creating new channels"
	case 91000:
		return "unable to enter presence channel (no clientId)"
	case 91001:
		return "unable to enter presence channel (invalid channel state)"
	case 91002:
		return "unable to leave presence channel that is not entered"
	case 91003:
		return "unable to enter presence channel (maximum member limit exceeded)"
	case 91004:
		return "unable to automatically re-enter presence channel"
	case 91005:
		return "presence state is out of sync"
	case 91100:
		return "member implicitly left presence channel (connection closed)"
	case 101000:
		return "must have a non-empty name for the space"
	case 101001:
		return "must enter a space to perform this operation"
	case 101002:
		return "lock request already exists"
	case 101003:
		return "lock is currently locked"
	case 101004:
		return "lock was invalidated by a concurrent lock request which now holds the lock"
	}
	return ""
}
