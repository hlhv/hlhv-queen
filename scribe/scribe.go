package scribe

import (
        "log"
)

type MessageType int

const (
        Progress MessageType = iota
        Done
        Info
        Warning
        Error
        Fatal
        Request
        Resolve
        Connect
        Mount
        Disconnect
        Unmount
        Bind
        Unbind
)

type Message struct {
        Type    MessageType
        Content []interface{}
}

var queue chan Message = make(chan Message, 16)

func ListenOnce () {
        message := <- queue

        content := message.Content
        content = append(content, "")
        copy(content[1:], content)

        switch message.Type {
        case Progress:
                content[0] = "..."
                log.Println(content...)
                break
        case Done:
                content[0] = ".//"
                log.Println(content...)
                break
        case Info:
                content[0] = "(i)"
                log.Println(content...)
                break
        case Warning:
                content[0] = "!!!"
                log.Println(content...)
                break
        case Error:
                content[0] = "ERR"
                log.Println(content...)
                break
        case Fatal:
                content[0] = "XXX"
                log.Fatalln(content...)
                break
        case Request:
                content[0] = "->?"
                log.Println(content...)
                break
        case Resolve:
                content[0] = "->!"
                log.Println(content...)
                break
        case Connect:
                content[0] = "-->"
                log.Println(content...)
                break
        case Mount:
                content[0] = "-=E"
                log.Println(content...)
                break
        case Disconnect:
                content[0] = "<--"
                log.Println(content...)
                break
        case Unmount:
                content[0] = "X=-"
                log.Println(content...)
                break
        case Bind:
                content[0] = "=#="
                log.Println(content...)
                break
        case Unbind:
                content[0] = "=X="
                log.Println(content...)
                break
        }
}

func Print (t MessageType, content ...interface{}) {
        queue <- Message {
                Type:    t,
                Content: content, 
        }
}

func PrintProgress   (content ...interface{}) { Print(Progress,   content...) }
func PrintDone       (content ...interface{}) { Print(Done,       content...) }
func PrintInfo       (content ...interface{}) { Print(Info,       content...) }
func PrintWarning    (content ...interface{}) { Print(Warning,    content...) }
func PrintError      (content ...interface{}) { Print(Error,      content...) }
func PrintFatal      (content ...interface{}) { Print(Fatal,      content...) }
func PrintRequest    (content ...interface{}) { Print(Request,    content...) }
func PrintResolve    (content ...interface{}) { Print(Resolve,    content...) }
func PrintConnect    (content ...interface{}) { Print(Connect,    content...) }
func PrintMount      (content ...interface{}) { Print(Mount,      content...) }
func PrintDisconnect (content ...interface{}) { Print(Disconnect, content...) }
func PrintUnmount    (content ...interface{}) { Print(Unmount,    content...) }
