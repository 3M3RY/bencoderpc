// Package bencoderpc implements Bencode based RPC. The following details are
// informative and not neccesary to use this package.
//
// Each Request contains a call identifier, a method string, and a parameters object
// in a bencode dictionary keyed by 'i', 'm', and 'p' respectively.
// A response contains an error string, a call identifier, and a result object in
// in a bencode dictionary keyed by 'e', 'i', and 'r' respectively.
// Call identifiers are not required to be integers.
package bencoderpc
