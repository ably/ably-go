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
// Calls to handlers for a single or for subsequent events may be concurrent,
// but are always ordered. That is, if an event is fired after another, the same
// handler will be called in the same order that the events were fired.
//
// Calling any of these methods an "off" function inside a handler will only
// have effect for subsequent events.
package ably
