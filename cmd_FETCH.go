package imap

import (
  "regexp"
  "fmt"
  "os"
  "strings"
)

type FetchOptions struct {
  body  bool
  // TODO: Body section
  // TODO: Body peek section
  bodystructure bool
  envelope bool
  flags bool
  internaldate bool
  rfc822 bool
  rfc822_header bool
  rfc822_size bool
  rfc822_text bool
  uid bool
}
var BodySectionRegexp = regexp.MustCompile("^BODY\\[([^\\]]+)\\](<[0-9]+\\.[0-9]+>)?")
var BodyPeekRegexp    = regexp.MustCompile("^BODY\\.PEEK\\[([^\\]]+)\\](<[0-9]+\\.[0-9]+>)?")

func fetch_consume_item(items string, opts FetchOptions) (string, error) {
  return "", nil
}

func fetch_parse_items(items string) FetchOptions {
  opts := FetchOptions{false, false, false, false, false, false, false, false, false, false}
  
  var err   error
  for (len(items) > 0) {
    items, err = fetch_consume_item(items, opts)
    if err != nil {
      fmt.Fprintf(os.Stderr, "ERROR in fetch_parse_items: %v\n", err)
    }
  }
  
  return opts
}

func (sess *Session) FETCH(comm *Command) error {
  parts := strings.SplitN(comm.Arguments, " ", 2)
  
  var items string//, seq string
  // seq = parts[0]
  if len(parts) == 2 {
    items = parts[1]
  } else {
    fmt.Fprintf(os.Stderr, "ERROR FETCH(%q): Missing data items\n", comm.Arguments)
    items = ""
  }
  // Utility
  up := func(s string) string { return strings.ToUpper(s) }
  items_up := up(items)
  
  if items_up == "ALL" {
    items = "(FLAGS INTERNALDATE RFC822.SIZE ENVELOPE)"
  } else if items_up == "FAST" {
    items = "(FLAGS INTERNALDATE RFC822.SIZE)"
  } else if items_up == "FULL" {
    items = "(FLAGS INTERNALDATE RFC822.SIZE ENVELOPE BODY)"
  }
  
  items, err := remove_parentheses_strict(items)
  if err != nil {
    sess.Sendf("%s BAD Invalid arguments: %s\r\n", comm.Tag, err.Error())
    return err
  }
  
  return nil
}
