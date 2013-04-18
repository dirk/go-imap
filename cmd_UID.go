package imap

import (
  "fmt"
  "os"
  "strings"
  "imap/intf"
)

func (sess *Session) UID(comm *Command) error {
  parts := strings.SplitN(comm.Arguments, " ", 2)
  if len(parts) != 2 {
    sess.Sendf("%s BAD Invalid command: %s\r\n", comm.Tag, comm.Command)
    return nil
  }
  subcommand := strings.ToUpper(parts[0])
  subargs    := parts[1]
  if subcommand == "FETCH" {
    return sess.UID_FETCH(comm, subargs)
  } else {
    sess.Sendf("%s BAD Invalid command: %s\r\n", comm.Tag, comm.Command)
    return nil
  }
  return nil
}
func (sess *Session) UID_FETCH(comm *Command, args string) error {
  parts := strings.SplitN(args, " ", 2)
  if len(parts) != 2 {
    sess.Sendf("%s BAD Invalid arguments: %s\r\n", comm.Tag, args)
    return nil
  }
  
  seq   := parts[0]
  // items := parts[1]
  
  msgs, err := FindUIDSequence(sess, comm, seq)
  if err != nil {
    sess.Sendf("%s NO Error fetching\r\n", comm.Tag)
    fmt.Fprintf(os.Stderr, "ERROR UID FETCH(%q): %v\n", comm.Arguments, err)
    return nil
  }
  
  if strings.Contains(args, "FLAGS") {
    for _, msg := range msgs {
      sess.Sendf("* %s FETCH (FLAGS ())\r\n", msg.GetUID())
    }
    sess.Sendf("%s OK FETCH\r\n", comm.Tag)
    return nil
  }
  
  sess.Sendf("%s BAD Not implemented\r\n", comm.Tag)
  fmt.Fprintf(os.Stderr, "WARN UID FETCH(%s)\n", args)
  return nil
}
func FindUIDSequence(sess *Session, comm *Command, seq string) ([]intf.Message, error) {
  if sess.mailbox == "" {
    sess.Sendf("%s NO No mailbox SELECT'ed\r\n", comm.Tag, seq)
    return nil, fmt.Errorf("ERROR UID FETCH(%q): %v", comm.Arguments, "No mailbox")
  }
  if strings.Contains(seq, ":") {
    parts := strings.SplitN(seq, ":", 2)
    if len(parts) != 2 {
      sess.Sendf("%s BAD Invalid sequence: %s\r\n", comm.Tag, seq)
      return nil, fmt.Errorf("Invalid sequence: %q", seq)
    }
    start_uid := parts[0]
    end_uid   := parts[1]
    if end_uid == "*" {
      return sess.Storage().MailboxFindMessagesAfterUID(sess.mailbox, start_uid)
    } else {
      fmt.Printf("FindMessagesFromTo not implemented\n")
      return sess.Storage().MailboxFindMessagesFromToUID(sess.mailbox, start_uid, end_uid)
    }
  } else {
    if seq == "*" {
      fmt.Printf("FindAllMessages not implemented\n")
      return sess.Storage().MailboxFindAllMessages(sess.mailbox)
    } else {
      fmt.Printf("FindMessage not implemented\n")
      msg, err := sess.Storage().MailboxFindMessageByUID(sess.mailbox, seq)
      return []intf.Message{msg}, err
    }
  }
  return nil, fmt.Errorf("Unreachable")
}
