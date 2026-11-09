package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
)

// The config file holds standing flag values: one JSON object, written
// by the game only at the settings panel's Done (settings.go:
// writeConfig, its four sound keys):
//
//	{
//	  "game": "/home/me/dosbox/Ascend",
//	  "volume": 0.4,
//	  "music-gap": "90s"
//	}
//
// A key is a flag's name, its value a JSON string or a bare number or
// bool; the command line wins. The sound keys (music, volume, music-gap,
// sfx: the settings panel's) are flags of their own set, never on the
// command line, so what the panel writes is what the next launch plays.
// The default ./aiscendancy.json may be absent; a file named by -config
// must exist.
const configDefault = "aiscendancy.json"

// applyConfig reads path into fset's flags left unset by the command line
// and into file's (the file-only flags); explicit says -config named it.
func applyConfig(fset, file *flag.FlagSet, path string, explicit bool) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	return parseConfig(fset, file, path, f)
}

// parseConfig applies r's object to fset and file (name for errors),
// refusing an unknown key, "config" itself and a bad value; sorted keys
// make the first refusal independent of the file's order.
func parseConfig(fset, file *flag.FlagSet, name string, r io.Reader) error {
	set := map[string]bool{}
	fset.Visit(func(f *flag.Flag) { set[f.Name] = true })
	var m map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		into := fset
		if into.Lookup(k) == nil {
			into = file
		}
		if k == "config" || into.Lookup(k) == nil {
			return fmt.Errorf("%s: no flag %q", name, k)
		}
		if set[k] {
			continue
		}
		// A string's text, else the JSON value as written (0.6, true).
		v := string(m[k])
		if len(v) > 0 && v[0] == '"' {
			if err := json.Unmarshal(m[k], &v); err != nil {
				return fmt.Errorf("%s: %s: %v", name, k, err)
			}
		}
		if err := into.Set(k, v); err != nil {
			return fmt.Errorf("%s: %s: %v", name, k, err)
		}
	}
	return nil
}
