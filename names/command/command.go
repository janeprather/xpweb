//go:generate go run gen_command_names.go

// Package command provides known command names as string constants to limit repetition of string
// literals and the risk of typos that can't be caught during lint/compile.
package command
