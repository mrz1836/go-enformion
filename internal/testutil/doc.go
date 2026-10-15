// Package testutil holds the test plumbing go-enformion's tests share: testdata
// fixtures, HTTP fakes and a recording TLS server, formatting-leak assertions, a
// token-walk JSON oracle, race-aware allocation guards, and the value-free
// shape printer the live probe reports with.
//
// It is imported only by tests. It imports testify and the standard library,
// never the enformion package, so that the root package's internal tests can
// use it too.
package testutil
