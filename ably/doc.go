// Package ably is the official Ably client library for Go.
//
// Get started at https://github.com/ably/ably-go#using-the-realtime-api.
//
// Event Emitters
//
// An event emitter pattern appears in multiple places in the library.
//
// The On method takes an event identifier, which can be nil, and a handler
// function to be called whenever an event is emitted in the event emitter
// with the event's associated data. It also returns an "off" function to undo
// this operation, so that the handler function isn't called anymore. If no
// event identifier is provided, the handler is called for all kinds of event.
//
// The Once method works like On, except the handler is just called once, for
// the first matching event.
//
// The Off method is like calling all the "off" function returned by On and Once
// methods. It takes an event identifier, which, if not nil, makes the effect
// like calling the "off" functions only from calls to On and Once with that
// specific event identifier as argument.
//
// Calls to handlers for a single or for subsequent events may be concurrent,
// but are always ordered. That is, if an event is fired after another, the same
// handler will be called in the same order that the events were fired.
//
// Calling On, Once, Off or an "off" function inside a handler will only have
// effect for subsequent events.
package ably
