package lunk

import "os"

func Example() {
	l := NewJSONEventLogger(os.Stdout)

	rootID := NewRootEventID()
	l.Log(rootID, Message("root action"))

	subID := NewEventID(rootID)
	l.Log(subID, Message("sub action"))

	leafID := NewEventID(subID)
	l.Log(leafID, Message("leaf action"))

	// Produces something like this:
	// {
	//     "properties": {
	//         "msg": "root action"
	//     },
	//     "pid": 44345,
	//     "host": "server1.example.com",
	//     "time": "2014-04-28T13:58:32.201883418-07:00",
	//     "id": "09c84ee90e7d9b74",
	//     "root": "ca2e3c0fdfcf3f5e",
	//     "schema": "message"
	// }
	// {
	//     "properties": {
	//         "msg": "sub action"
	//     },
	//     "pid": 44345,
	//     "host": "server1.example.com",
	//     "time": "2014-04-28T13:58:32.202241745-07:00",
	//     "parent": "09c84ee90e7d9b74",
	//     "id": "794f8bde67a7f1a7",
	//     "root": "ca2e3c0fdfcf3f5e",
	//     "schema": "message"
	// }
	// {
	//     "properties": {
	//         "msg": "leaf action"
	//     },
	//     "pid": 44345,
	//     "host": "server1.example.com",
	//     "time": "2014-04-28T13:58:32.202257354-07:00",
	//     "parent": "794f8bde67a7f1a7",
	//     "id": "33cff19e8bfb7cef",
	//     "root": "ca2e3c0fdfcf3f5e",
	//     "schema": "message"
	// }
}
