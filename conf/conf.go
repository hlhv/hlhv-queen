package conf

import (
        "io"
        "os"
        "sync"
        "bufio"
        "strings"
        "strconv"
        "unicode"
        "github.com/hlhv/hlhv/scribe"
)

type databaseType struct {
        keyPath   string
        certPath  string
        connKey   string
        timeout   int

        portHlhv  int
        portHttps int
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

var (
        confpath string = "/etc/hlhv/hlhv.conf"
)

func Load () (err error) {
        scribe.PrintProgress( "reading config file")

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
                timeout:   5000,
                
                portHlhv:  2001,
                portHttps: 443,
        }

        // TODO: read file :P
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
                        "using alias (fallback) -> " + aliases.fallback)
        }

        for key, val = range aliases.database {
                scribe.PrintInfo (
                        "using alias " + key + " -> " + val)
        }

        return nil
}

func handleKeyVal (key string, val string) {
        valn, _ := strconv.Atoi(val)

        switch key {
                case "alias": {
                        aliasSplit := strings.SplitN(val, "->", 2)
                        left  := strings.TrimSpace(aliasSplit[0])
                        right := strings.TrimSpace(aliasSplit[1])
                        
                        if len(left) < 1 || len(right) < 1 { break }

                        if left == "(fallback)" {
                                aliases.fallback = right
                        } else {
                                aliases.database[left] = right
                        }

                }; break
                
                case "keyPath":   items.database.keyPath   = val;  break
                case "certPath":  items.database.certPath  = val;  break
                case "connKey":   items.database.connKey   = val;  break
                case "timeout":   items.database.timeout   = valn; break
                case "portHlhv":  items.database.portHlhv  = valn; break
                case "portHttps": items.database.portHttps = valn; break
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
