// Copyright 2015 Brett Vickers.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package etree

import "os"

// Create an etree Document, add XML entities to it, and serialize it
// to stdout.
func ExampleDocument_creating() {
	doc := NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
	doc.CreateProcInst("xml-stylesheet", `type="text/xsl" href="style.xsl"`)

	people := doc.CreateElement("People")
	people.CreateComment("These are all known people")

	jon := people.CreateElement("Person")
	jon.CreateAttr("name", "Jon O'Reilly")

	sally := people.CreateElement("Person")
	sally.CreateAttr("name", "Sally")

	doc.Indent(2)
	doc.WriteTo(os.Stdout)
	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <?xml-stylesheet type="text/xsl" href="style.xsl"?>
	// <People>
	//   <!--These are all known people-->
	//   <Person name="Jon O&apos;Reilly"/>
	//   <Person name="Sally"/>
	// </People>
}

func ExampleDocument_reading() {
	doc := NewDocument()
	if err := doc.ReadFromFile("document.xml"); err != nil {
		panic(err)
	}
}

func ExamplePath() {
	xml := `<bookstore><book><title>Great Expectations</title>
      <author>Charles Dickens</author></book><book><title>Ulysses</title>
      <author>James Joyce</author></book></bookstore>`

	doc := NewDocument()
	doc.ReadFromString(xml)
	for _, e := range doc.FindElements(".//book[author='Charles Dickens']") {
		doc := NewDocument()
		doc.SetRoot(e.Copy())
		doc.Indent(2)
		doc.WriteTo(os.Stdout)
	}
	// Output:
	// <book>
	//   <title>Great Expectations</title>
	//   <author>Charles Dickens</author>
	// </book>
}
