package save

import "os"

// dataDir is %LOCALAPPDATA% (C:\Users\<name>\AppData\Local), "" unset.
func dataDir() string { return os.Getenv("LOCALAPPDATA") }
