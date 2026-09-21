// Package execid derives the basic execution identity the wrapper reports for a run.
//
// The wrapper only sees argv. Per the resources-sizing spec the arguments may hold sensitive
// data, so only the command name travels in clear; the arguments are reduced to a short hash.
// Software name, version and the call-site line number are the backend's and the SDK's to add.
package execid

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"
)

// HashLength is how many hex characters of the argument digest are kept.
const HashLength = 12

// Exec is the execution identity derived from a command line.
type Exec struct {
	// Command is the base name of argv[0].
	Command string `json:"command"`
	// ArgsHash is the first HashLength hex characters of SHA-256 over argv[1:], NUL-separated.
	ArgsHash string `json:"argsHash"`
	// ArgCount is len(argv) - 1.
	ArgCount int `json:"argCount"`
	// Mode says how the command reached the wrapper: "argv" (after `--`) or "shell"
	// (PL_JOB_CMD_AND_ARGS, tokenized for identification only).
	Mode string `json:"mode,omitempty"`
}

// Derive builds an Exec from argv. ok is false for an empty argv.
func Derive(argv []string) (e Exec, ok bool) {
	if len(argv) == 0 {
		return Exec{}, false
	}
	h := sha256.New()
	for _, a := range argv[1:] {
		h.Write([]byte(a))
		h.Write([]byte{0})
	}
	return Exec{
		Command:  path.Base(filepath.ToSlash(argv[0])),
		ArgsHash: hex.EncodeToString(h.Sum(nil))[:HashLength],
		ArgCount: len(argv) - 1,
	}, true
}

// ID renders the identity as "<command>:<argsHash>", or "" for a zero Exec.
func (e Exec) ID() string {
	if e.Command == "" {
		return ""
	}
	return e.Command + ":" + e.ArgsHash
}
