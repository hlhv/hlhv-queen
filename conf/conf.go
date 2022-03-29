package conf

import (
        "io"
        "os"
        "sync"
        "bufio"
        "strings"
        "strconv"
        "unicode"
        "github.com/hlhv/scribe"
)

type databaseType struct {
        keyPath   string
        certPath  string
        connKey   string

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
        database map[string] string
        mutex    sync.RWMutex
}

func Load (confpath string) (err error) {
        scribe.PrintProgress(scribe.LogLevelNormal, "reading config file")

        items.mutex.RLock()
        aliases.mutex.RLock()
        defer aliases.mutex.RUnlock()
        defer items.mutex.RUnlock()
        
        // default aliases
        aliases.database = map[string] string {
                "localhost":        "@",
                "127.0.0.1":        "@",
                "::ffff:127.0.0.1": "@",
                "::1":              "@",
        }

        // default configuration items
        items.database = databaseType {
                keyPath:   "/var/hlhv/cert/key.pem",
                certPath:  "/var/hlhv/cert/cert.pem",
                
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
        if err != nil { return err }
        reader := bufio.NewReader(file)

        var key   string
        var val   string
        var state int
        for {
                ch, _, err := reader.ReadRune()
                if err != nil {
                        if err == io.EOF { break }
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
                                key   = ""
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
                                key   = ""
                                val   = ""
                                state = 0
                        } else {
                                val += string(ch)
                        }
                        break

                case 3:
                        // ignore comment until EOL
                        if ch == '\n' { state = 0 }
                        break
                }
        }

        file.Close()

        if aliases.fallback != "" {
                scribe.PrintInfo (
                        scribe.LogLevelDebug,
                        "using alias (fallback) -> " + aliases.fallback)
        }

        for key, val = range aliases.database {
                scribe.PrintInfo (
                        scribe.LogLevelDebug,
                        "using alias " + key + " -> " + val)
        }

        return nil
}

func handleKeyVal (key string, val string) {
        valn, _ := strconv.Atoi(val)

        switch key {
                case "alias": {
                        aliasSplit := strings.SplitN(val, "->", 2)
                        if len(aliasSplit) < 2 { break }
                        left  := strings.TrimSpace(aliasSplit[0])
                        right := strings.TrimSpace(aliasSplit[1])
                        
                        if len(left) < 1 || len(right) < 1 { break }

                        if left == "(fallback)" {
                                aliases.fallback = right
                        } else {
                                aliases.database[left] = right
                        }

                }; break
                
                case "keyPath":    items.database.keyPath    = val;  break
                case "certPath":   items.database.certPath   = val;  break
                case "connKey":    items.database.connKey    = val;  break
                case "portHlhv":   items.database.portHlhv   = valn; break
                case "portHttps":  items.database.portHttps  = valn; break
                case "gardenFreq": items.database.gardenFreq = valn; break
                case "maxBandAge": items.database.maxBandAge = valn; break
                case "timeout":    items.database.timeout    = valn; break
                
                case "timeoutReadHeader":
                        items.database.timeoutReadHeader = valn
                        break
                case "timeoutRead":
                        items.database.timeoutRead = valn
                        break
                case "timeoutWrite":
                        items.database.timeoutWrite = valn
                        break
                case "timeoutIdle":
                        items.database.timeoutIdle = valn
                        break
        }
}

func ResolveAliases (input string) (output string) {
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
