// Package stamp implements the Simple Two-way Active Measurement Protocol
// (STAMP) as defined in RFC 8762.
//
// It provides two layers:
//
//   - A low-level packet codec: Packet, Encode, Decode.
//   - A high-level session API: Sender and Reflector.
//
// Only the unauthenticated base mode is supported in v0. STAMP Optional
// Extensions (RFC 8972) and HMAC authentication are out of scope for now.
package stamp
