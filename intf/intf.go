package intf

import (
  SMTP "smtp"
  "time"
)

type Mailbox interface {
  Name() string
}
type Message interface {
  UID()        string
  Flags()      []string
  From()       *SMTP.Address
  Date()       time.Time
  Size()       uint32
  HeaderSize() uint32
  BodySize()   uint32
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
