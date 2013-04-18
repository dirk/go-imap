package imap

import (
  go1_imap "github.com/sbinet/go-imap/go1/imap"
  "strings"
  "fmt"
)

func remove_parentheses_strict(s string) (string, error) {
  if strings.HasPrefix(s, "(") {
    if strings.HasSuffix(s, ")") {
      return s[1:(len(s) - 2)], nil
    } else {
      return "", fmt.Errorf("ERROR FETCH(ITEMS: %q): Missing matching trailing parentheses", s)
    }
  }
  return s, nil
}

func quote(s string) string {
  return go1_imap.Quote(s, false)
}
func unquote(s string) string {
  if go1_imap.Quoted(s) {
    r, ok := go1_imap.Unquote(s)
    if ok {
      return r
    } else { return s }
  }
  return s
}

