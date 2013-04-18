package imap

import (
  "bufio"
  "fmt"
  "net"
  "strings"
  // "time"
  go1_imap "github.com/sbinet/go-imap/go1/imap"
  "time"
  "os"
  "strconv"
  "imap/intf"
)

// SERVER ---------------------------------------------------------------------

const HIERARCHY_DELIMITER = "/"

type LoginProvider func(*Session, string, string) error

type Server struct {
  debug bool
  addr string
  hostname string
  listener net.Listener
  closed bool
  Login LoginProvider
}
func (server *Server) IsDebug() bool {
  return server.debug
}
func (server *Server) SetDebug(d bool) {
  server.debug = d
}
func (server *Server) Closed() bool { return server.closed }
func (server *Server) Close() {
  server.closed = true
  server.listener.Close()
}
func NewServer(hostname string, addr string, login_provider LoginProvider) *Server {
  server := &Server{false, addr, hostname, nil, false, login_provider}
  return server
}

// SESSION --------------------------------------------------------------------

const (
  NOT_AUTHENTICATED = 0
  AUTHENTICATED = 1
)

type Session struct {
  server *Server
  conn net.Conn
  reader *bufio.Reader
  writer *bufio.Writer
  // Stateful stuff
  state int
  username string
  postbox string // Name of the postbox for this session
  mailbox string // Name of the mailbox
  storage intf.Storage // Storage system
}
func NewSession(
  server *Server, conn net.Conn,
  reader *bufio.Reader, writer *bufio.Writer,
) *Session {
  s := &Session{server, conn, reader, writer, NOT_AUTHENTICATED, "", "", "", nil}
  return s
}
func (sess *Session) Sendf(format string, args ...interface{}) {
  fmt.Fprintf(sess.writer, format, args...)
  sess.writer.Flush()
}
func (sess *Session) Readline() (string, error) {
  s, e := sess.reader.ReadString('\n')
  return s, e
}
func (sess *Session) Storage() intf.Storage {
  return sess.storage
}
func (sess *Session) SetStorage(storage intf.Storage) {
  sess.storage = storage
}
func (sess *Session) SetUsername(username string) {
  sess.username = username
}

// COMMAND --------------------------------------------------------------------

type Command struct {
  Tag string
  Command string
  Arguments string
}
func ParseCommand(s string) (*Command, error) {
  var tag, com, args string
  
  ti := strings.Index(s, " ")
  if ti == -1 {
    return nil, fmt.Errorf("Missing tag in command %q", s)
  } else {
    tag = s[:ti]
    s = s[ti + 1:]
  }
  // TODO: Ensure this is bounds-bulletproof
  ci := strings.Index(s, " ")
  if ci == -1 {
    com = strings.ToUpper(strings.TrimSpace(s))
    s = ""
    if com == "" {
      return nil, fmt.Errorf("Missing command after tag %q", s)
    }
  } else {
    com = strings.ToUpper(strings.TrimSpace(s[:ci]))
    s = s[ci + 1:]
  }
  args = s
  
  command := &Command{tag, com, args}
  return command, nil
}

type CommandNotFoundError struct {
  Message string
}
func (e CommandNotFoundError) Error() string {
  return e.Message
}


func handle_session(sess *Session) error {
  // TODO: Timeouts
  timeout := time.Duration(30) * time.Minute;
  sess.conn.SetReadDeadline(time.Now().Add(timeout))
  
  if sess.server.IsDebug() { fmt.Printf("OPENED %p\n", sess) }
  
  // Send greeting
  sess.Sendf("OK %s IMAP4rev1\r\n", sess.server.hostname)
  
  var command *Command
  
command:
  s, e := sess.Readline()
  if e != nil { goto err }
  s = strings.TrimRight(s, "\r\n")
  if sess.server.IsDebug() { fmt.Printf("COMMAND: %s\n", s) }
  
  command, e = ParseCommand(s)
  if e != nil { goto err }
  
  // Handle commands
  
  // ANY STATE
  switch command.Command {
  case "CAPABILITY":
    // e = fmt.Errorf("CAPABILITY not implemented yet")
    // goto err
    sess.Sendf("* CAPABILITY IMAP4rev1\r\n")
    sess.Sendf("%s OK CAPABILITY\r\n", command.Tag)
    goto command
  case "NOOP":
    sess.Sendf("%s OK\r\n", command.Tag)
    goto command
  case "LOGOUT":
    sess.Sendf("* BYE %s\r\n", sess.server.hostname)
    sess.Sendf("%s OK LOGOUT\r\n", command.Tag)
    goto close
  }
  
  // Default error to be overwritten by response from handle_'s
  // e = fmt.Errorf("Unrecognized command: %s", command.Command)
  e = CommandNotFoundError{fmt.Sprintf("Unrecognized command: %s", command.Command)}
  
  if sess.state == NOT_AUTHENTICATED {
    // NOT AUTHENTICATED --------------------------------------
    switch command.Command {
    case "LOGIN":
      e = sess.LOGIN(command)
    }
  } else if sess.state == AUTHENTICATED {
    // AUTHENTICATED ------------------------------------------
    switch command.Command {
    case "SELECT":
      e = sess.SELECT(command)
    case "LSUB":
      e = sess.LSUB(command)
    case "LIST":
      e = sess.LIST(command)
    case "CLOSE":
      // TODO: \Deleted message cleanup and such
      sess.Sendf("%s OK CLOSING\r\n", command.Tag)
      goto close
    case "CREATE":
      e = sess.CREATE(command)
    case "UID":
      e = sess.UID(command)
    case "STATUS":
      e = sess.STATUS(command)
    case "FETCH":
      e = sess.FETCH(command)
    case "CHECK":
      // TODO: Make this do stuff
      e = nil
      sess.Sendf("%s OK CHECK\r\n", command.Tag)
    }//switch Command
  }
  
  if _, ok := e.(CommandNotFoundError); ok {
    sess.Sendf("%s BAD Invalid command: %s\r\n", command.Tag, command.Command)
    goto command
  }
  
  if e != nil {
    goto err
  } else { goto command }
  
  goto close
  
  // fmt.Printf("command: %v\n", command)
  
close:
  sess.conn.Close()
  if sess.server.IsDebug() { fmt.Printf("CLOSED %p\n\n", sess) }
  return nil
  
err:
  sess.conn.Close()
  return fmt.Errorf("handle_session: %v", e)
}


func (sess *Session) LSUB(comm *Command) error {
  mbs, err := sess.Storage().GetMailboxes()
  // TODO: Parse arguments
  if err != nil { return err }
  for _, mb := range mbs {
    // fmt.Printf("mb: %+v\n", mb)
    sess.Sendf(
      "* LIST () \"%s\" %s\r\n",
      HIERARCHY_DELIMITER, go1_imap.Quote(mb.Name(), false),
    )
  }
  sess.Sendf("%s OK LSUB\r\n", comm.Tag)
  return nil
}

func (sess *Session) CREATE(comm *Command) error {
  mailbox := unquote(strings.TrimSpace(comm.Arguments))
  if strings.ToUpper(mailbox) == "INBOX" {
    sess.Sendf("%s NO Can't CREATE an INBOX\r\n", comm.Tag)
    return nil
  }
  mb, e := sess.Storage().NewMailbox(mailbox)
  if e != nil {
    sess.Sendf("%s NO Failed to create mailbox\r\n", comm.Tag)
    return e
  } else {
    sess.Sendf("%s OK Created mailbox '%s'\r\n", comm.Tag, mb.Name())
  }
  return nil
}

const SYSTEM_FLAGS = "\\Seen \\Answered \\Flagged \\Deleted \\Draft \\Recent"

func (sess *Session) SELECT(comm *Command) error {
  // Reset first (according to RFC)
  sess.mailbox = ""
  // Filter the parameter
  m_name := strings.TrimSpace(comm.Arguments)
  m_name = unquote(m_name)
  
  // Look up the mailbox
  _, err := sess.Storage().GetMailbox(m_name)
  if err != nil {
    fmt.Fprintf(os.Stderr, "SELECT(%s) Error: %v\n", m_name, err)
    sess.Sendf("%s BAD Could not open mailbox\r\n")
    return nil
  }
  sess.mailbox = m_name
  
  fmt.Printf("SELECT(%s) Success\n", m_name)
  sess.Sendf("* FLAGS (%s)\r\n", SYSTEM_FLAGS)
  count, err := sess.Storage().MailboxCountAllMessages(m_name)
  if err != nil {
    fmt.Fprintf(os.Stderr, "WARN SELECT(%s): MailboxCountAllMessages: %v\n", comm.Arguments, err)
    sess.Sendf("* 0 EXISTS\r\n")
  } else {
    sess.Sendf("* %d EXISTS\r\n", count)
  }
  sess.Sendf("* 0 RECENT\r\n")
  // sess.Sendf("* OK [UNSEEN 0]\r\n") // FIXME: Naughty
  uid, err := sess.Storage().GetUID()
  if err != nil { panic(err) }
  sess.Sendf("* OK [UIDNEXT %s] Next UID\r\n", uid)
  sess.Sendf("* OK [UIDVALIDITY 0] Next UID\r\n")
  sess.Sendf("%s OK [READ-WRITE] SELECT\r\n", comm.Tag)
  return nil
}
func (sess *Session) LIST(comm *Command) error {
  parts := strings.SplitN(comm.Arguments, " ", 2)
  ref_name := parts[0]
  if ref_name != "\"\"" {
    sess.Sendf("%s BAD Reference names not allowed\r\n")
    return nil
  }
  mailbox := unquote(strings.TrimSpace(parts[1]))
  if mailbox == "*" {
    var mbs []intf.Mailbox
    mbs, err := sess.Storage().GetMailboxes()
    if err != nil { return err }
    for _, mb := range mbs {
      sess.Sendf("* LIST () \"%s\" %s\r\n", HIERARCHY_DELIMITER, quote(mb.Name()))
    }
    sess.Sendf("%s OK LIST\r\n", comm.Tag)
  } else if strings.ToUpper(mailbox) == "INBOX" {
    sess.Sendf("* LIST () \"%s\" \"INBOX\"\r\n", HIERARCHY_DELIMITER)
    sess.Sendf("%s OK LIST\r\n", comm.Tag)
  } else {
    mb, err := sess.Storage().GetMailbox(mailbox)
    if err != nil { return err }
    if mb != nil {
      sess.Sendf(
        "* LIST () \"%s\" %s\r\n",
        HIERARCHY_DELIMITER, go1_imap.Quote(mb.Name(), false),
      )
      sess.Sendf("%s OK LIST\r\n", comm.Tag)
    } else {
      sess.Sendf("%s NO\r\n", comm.Tag);
    }
  }
  return nil
}

func (sess *Session) STATUS(comm *Command) error {
  parts := strings.SplitN(comm.Arguments, " ", 2)
  m_name  := unquote(parts[0])
  p_items := parts[1]
  
  mb, err := sess.Storage().GetMailbox(m_name)
  if err != nil {
    fmt.Fprintf(os.Stderr, "ERROR: STATUS(%s) %v\n", m_name, err)
    sess.Sendf("%s NO Mailbox doesn't exist\r\n", comm.Tag)
    return err
  }
  
  pairs := make([]string, 0)
  messages := (strings.Contains(p_items, "MESSAGES"))
  recent   := (strings.Contains(p_items, "RECENT"))
  uidnext  := (strings.Contains(p_items, "UIDNEXT"))
  uidvalidity := (strings.Contains(p_items, "UIDVALIDITY"))
  unseen   := (strings.Contains(p_items, "UNSEEN"))
  
  if messages {
    count, err := sess.Storage().MailboxCountAllMessages(mb.Name())
    if err != nil {
      fmt.Fprintf(os.Stderr, "ERROR: STATUS(%s) MESSAGES: %v\n", m_name, err)
    } else {
      pairs = append(pairs, "MESSAGES "+strconv.Itoa(count))
    }
  }
  if recent {
    // FIXME: Make this work
    pairs = append(pairs, "RECENT 0")
  }
  if uidnext {
    uid, err := sess.Storage().NextUID()
    if err != nil {
      fmt.Fprintf(os.Stderr, "ERROR: STATUS(%s) MESSAGES: %v\n", m_name, err)
    } else {
      pairs = append(pairs, "UIDNEXT "+uid)
    }
  }
  if uidvalidity {
    // FIXME: Make this work
    pairs = append(pairs, "UIDVALIDITY 0")
  }
  if unseen {
    // FIXME: Make this work
    pairs = append(pairs, "UNSEEN 0")
  }
  sess.Sendf("* STATUS %s (%s)\r\n", quote(mb.Name()), strings.Join(pairs, " "))
  sess.Sendf("%s OK STATUS\r\n", comm.Tag)
  return nil
}

func (sess *Session) LOGIN(comm *Command) error {
  pair := strings.SplitN(comm.Arguments, " ", 2)
  if pair == nil || len(pair) != 2 {
    return fmt.Errorf("Invalid LOGIN arguments: %q", comm.Arguments)
  }
  user := unquote(pair[0])
  pass := pair[1]
  
  // TODO: User authentication
  err := sess.server.Login(sess, user, pass)
  if err != nil { return err }
  
  if sess.username != "" {
    sess.username = user
    sess.state = AUTHENTICATED
    sess.Sendf("%s OK LOGIN\r\n", comm.Tag)
    fmt.Printf("Logged in: %q\n", user)
  } else {
    return fmt.Errorf("ERROR LOGIN\n")
  }
  
  return nil
}

// SERVER ---------------------------------------------------------------------

func Listen(server *Server) (error) {
  ln, e := net.Listen("tcp", server.addr)
  if e != nil {
    return e
  } else {
    server.listener = ln
  }
  return nil
}

func Serve(server *Server) error {
  for {
    conn, e := server.listener.Accept()
    if e != nil {
      if server.Closed() {
        break
      }
      fmt.Printf("accept error: %v\n", e)
      return e
    }
    go func(conn_pointer *net.Conn) {
      conn := *conn_pointer
      sess := NewSession(
        server, conn,
        bufio.NewReader(conn), bufio.NewWriter(conn),
      )
      
      e = handle_session(sess)
      if e != nil {
        fmt.Printf("Serve() ERROR: %v\n", e)
        return
      }
      
    }(&conn)//goroutine
  }
  
  return nil
}


