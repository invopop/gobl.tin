// Package api defines the Verifier port that register clients implement, the
// report types they fill, the Policy that turns a report into a patch, and
// the error taxonomy they use.
//
// ABOUT: This package sits below the verifiers so that both they and the root
// package can share these types without an import cycle. The root package
// aliases the user-facing types, so consumers normally only import
// github.com/invopop/gobl.tin.
package api
