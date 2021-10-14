// Package ably is the official Ably client library for Go.
//
// Get started at https://github.com/ably/ably-go#using-the-realtime-api.
//
// Event Emitters
//
// An event emitter pattern appears in multiple places in the library.
//
// The On method takes an event type identifier and a handler function to be
// called with the event's associated data whenever an event of that type is
// emitted in the event emitter. It also returns an "off" function to undo this
// operation, so that the handler function isn't called anymore.
//
// The OnAll method is like On, but for events of all types.
//
// The Once method works like On, except the handler is just called once, for
// the first matching event.
//
// OnceAll is like OnAll in the same way Once is like On.
//
// The Off method is like calling the "off" function returned by calls to On and
// Once methods with a matching event type identifier.
//
// The OffAll method is like Off, except it is like calling all the "off"
// functions.
//
// Each handler is assigned its own sequential queue of events. That is, any
// given handler function will not receive calls from different goroutines that
// run concurrently; you can count on the next call to a handler to happen
// after the previous call has returned, and you can count on events or
// messages to be delivered to the handler in the same order they were emitted.
// Different handlers may be called concurrently, though.
//
// Calling any of these methods an "off" function inside a handler will only
// have effect for subsequent events.
//
// For messages and presence messages, "on" is called "subscribe" and "off" is
// called "unsubscribe".
//
// Paginated results
//
// Most requests to the Ably REST API return a single page of results, with
// hyperlinks to the first and next pages in the whole collection of results.
// To facilitate navigating through these pages, the library provides access to
// such paginated results though a common pattern.
//
// A method that prepares a paginated request returns a Request object with two
// methods: Pages and Items. Pages returns a PaginatedResult, an iterator that,
// on each iteration, yields a whole page of results. Items is simply a
// convenience wrapper that yields single results instead.
//
// In both cases, calling the method validates the request and may return an
// error.
//
// Then, for accessing the results, the Next method from the resulting
// iterator object must be called repeatedly; each time it returns true, the
// result that has been retrieved can be inspected with the Items or Item method
// from the iterator object. Finally, once it returns false, the Err method must
// be called to check if the iterator stopped due to some error, or else, it
// just finished going through all pages.
//
// See the PaginatedResults example.
package ably
