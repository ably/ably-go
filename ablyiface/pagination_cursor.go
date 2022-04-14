 package ablyiface
// import(
// 	"context"
// 	ably "github.com/ably/ably-go/ably"
// )

// For client.Stats()

// listing all items returns:
//type StatsPaginatedItems struct 
// which has methods
// 	Next(ctx context.Context) bool
//  Item() *Stats

// listing all pages returns:
// type StatsPaginatedResult struct
// which has methods
// Next(ctx context.Context) bool
// IsLast(ctx context.Context) bool
//  HasNext(ctx context.Context) bool
//  Items() []*Stats


// For client.History()

// listing all items returns:
// type MessagesPaginatedItems struct 
// which has methods
// Next(ctx context.Context) bool
// Item() *Message

// listing all pages returns
// type MessagesPaginatedResult struct 
// which has methods
// Next(ctx context.Context) bool
// IsLast(ctx context.Context) bool
// HasNext(ctx context.Context) bool
//  Items() []*Message

// For client.GetPresence()

// listing all items returns:
// PresencePaginatedItems struct
// which has methods
// Next(ctx context.Context) bool
// Item() *PresenceMessage

// Listing all pages returns:
// type PresencePaginatedResult struct 
// which has methods
// Next(ctx context.Context) bool
// IsLast(ctx context.Context) bool
// HasNext(ctx context.Context) bool
// Items() []*PresenceMessage



// //Pagination cursor is an interface that pagination cursors can pass through
// type PaginationCursor interface {
// 	Next(ctx context.Context) bool

// }