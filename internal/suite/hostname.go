package suite

import "os"

func osHostname() (string, error) { return os.Hostname() }
