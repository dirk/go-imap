package intf

import (
  SMTP "smtp"
  "time"
  "fmt"
)

// ERRORS ---------------------------------------------------------------------

type MailboxNotFound struct {
  name string
  message string
}
func (mnf MailboxNotFound) Error() string {
  if mnf.message == "" {
    return fmt.Sprintf("Mailbox %q not found", mnf.name)
  }
  return fmt.Sprintf("Mailbox %q not found: %s", mnf.name, mnf.message)
}
func NewMailboxNotFound(name, message string) MailboxNotFound {
  return MailboxNotFound{name, message}
}

// INTERFACES -----------------------------------------------------------------

type Mailbox interface {
  Name() string
}
type Message interface {
  GetUID()        string
  GetFlags()      []string
  GetFrom()       *SMTP.Address
  GetDate()       time.Time
  GetSize()       uint32
  GetHeaderSize() uint32
  GetBodySize()   uint32
}
type Envelope interface {
  From() *SMTP.Address
  To()   []*SMTP.Address
}

type Storage interface {
  SetUsername(username string)
  GetUsername() string
  
  // Mailbox creation
  NewMailbox(mailbox string) (Mailbox, error)
  // Mailbox commands
  MailboxCountAllMessages(mailbox string) (int, error)
  MailboxFindMessagesAfterUID(mailbox string, start_uid string) ([]Message, error)
  MailboxFindMessagesFromToUID(mailbox string, start string, end string) ([]Message, error)
  MailboxFindAllMessages(mailbox string) ([]Message, error)
  MailboxFindMessageByUID(mailbox string, uid string) (Message, error)
  
  // Storage commands
  GetMailboxes() ([]Mailbox, error)
  GetMailbox(mailbox string) (Mailbox, error)
  
  GetUID() (string, error)
  NextUID() (string, error)
  
}
