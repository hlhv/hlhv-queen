package conf

import (
	"bufio"
	"github.com/hlhv/scribe"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type databaseType struct {
	keyPath  string
	certPath string
	connKey  string

	portHlhv  int
	portHttps int

	gardenFreq int
	maxBandAge int

	timeout           int
	timeoutReadHeader int
	timeoutRead       int
	timeoutWrite      int
	timeoutIdle       int
}

var items struct {
	database databaseType
	mutex    sync.RWMutex
}

var aliases struct {
	fallback string
	database map[string]string
	mutex    sync.RWMutex
}

func Load(confpath string) (err error) {
	scribe.PrintProgress(scribe.LogLevelNormal, "reading config file")

	items.mutex.RLock()
	aliases.mutex.RLock()
	defer aliases.mutex.RUnlock()
	defer items.mutex.RUnlock()

	// default aliases
	aliases.database = map[string]string{
		"localhost":        "@",
		"127.0.0.1":        "@",
		"::ffff:127.0.0.1": "@",
		"::1":              "@",
	}

	// default configuration items
	items.database = databaseType{
		keyPath:  "/var/hlhv/cert/key.pem",
		certPath: "/var/hlhv/cert/cert.pem",
		connKey:  "",

		portHlhv:  2001,
		portHttps: 443,

		gardenFreq: 120,
		maxBandAge: 60,

		timeout:           1,
		timeoutReadHeader: 5,
		timeoutRead:       10,
		timeoutWrite:      15,
		timeoutIdle:       120,
	}

	file, err := os.OpenFile(confpath, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(file)

	var key string
	var val string
	var state int
	for {
		ch, _, err := reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch state {
		case 0:
			// wait for key or comment
			if ch == '#' {
				state = 3
			} else if !unicode.IsSpace(ch) {
				state = 1
				reader.UnreadRune()
			}
			break

		case 1:
			// ignore whitespace until value (or EOL)
			if ch == '\n' {
				key = ""
				state = 0
			} else if unicode.IsSpace(ch) {
				state = 2
			} else {
				key += string(ch)
			}
			break

		case 2:
			// get key until EOL
			if ch == '\n' {
				handleKeyVal(key, strings.TrimSpace(val))
				key = ""
				val = ""
				state = 0
			} else {
				val += string(ch)
			}
			break

		case 3:
			// ignore comment until EOL
			if ch == '\n' {
				state = 0
			}
			break
		}
	}

	file.Close()
	analyzeConfig()
	return nil
}

func analyzeConfig() {
	if aliases.fallback != "" {
		scribe.PrintInfo(
			scribe.LogLevelDebug,
			"using alias (fallback) -> "+aliases.fallback)
	}

	for key, val := range aliases.database {
		scribe.PrintInfo(
			scribe.LogLevelDebug,
			"using alias "+key+" -> "+val)
	}

	if items.database.connKey == "" {
		scribe.PrintWarning(
			scribe.LogLevelError,
			"CONNECTION KEY WAS NOT SET, SYSTEM IS VULNERABLE TO "+
				"ATTACK!",
		)
	}
}

func handleKeyVal(key string, val string) {
	valn, _ := strconv.Atoi(val)

	switch key {
	case "alias":   parseAlias(key, val)
	case "unalias": delete(aliases.database, val)

	case "keyPath":           items.database.keyPath = val
	case "certPath":          items.database.certPath = val
	case "connKey":           items.database.connKey = val
	case "portHlhv":          items.database.portHlhv = valn
	case "portHttps":         items.database.portHttps = valn
	case "gardenFreq":        items.database.gardenFreq = valn
	case "maxBandAge":        items.database.maxBandAge = valn
	case "timeout":           items.database.timeout = valn
	case "timeoutReadHeader": items.database.timeoutReadHeader = valn
	case "timeoutRead":       items.database.timeoutRead = valn
	case "timeoutWrite":      items.database.timeoutWrite = valn
	case "timeoutIdle":       items.database.timeoutIdle = valn
	}
}

func parseAlias(key string, val string) {
	aliasSplit := strings.SplitN(val, "->", 2)
	if len(aliasSplit) < 2 {
		return
	}
	left := strings.TrimSpace(aliasSplit[0])
	right := strings.TrimSpace(aliasSplit[1])

	if len(left) < 1 || len(right) < 1 {
		return
	}

	if left == "(fallback)" {
		aliases.fallback = right
	} else {
		aliases.database[left] = right
	}
}

func ResolveAliases(input string) (output string) {
	aliases.mutex.RLock()
	defer aliases.mutex.RUnlock()

	// try to match an alias
	for key, value := range aliases.database {
		if input == key {
			return value
		}
	}

	// if a fallback is set, and no aliases were found, use fallback
	if aliases.fallback != "" {
		return aliases.fallback
	}

	// if we don't have anything to resolve, return input as is
	return input
}
