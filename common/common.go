package common

// Less defines a function that compares the order of a and b.
// Returns true if a < b
type Less func(a, b interface{}) bool
