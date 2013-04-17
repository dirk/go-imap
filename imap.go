package imap

import (
  "bufio"
  "fmt"
  "net"
  "strings"
  // "time"
  "postoffice/postbox"
  "path"
  go_imap "github.com/sbinet/go-imap/go1/imap"
  "time"
)

// SERVER ---------------------------------------------------------------------

const HIERARCHY_DELIMITER = "/"

type Server struct {
  debug bool
  addr string
  hostname string
  listener net.Listener
}
func (server *Server) IsDebug() bool {
  return server.debug
}
func (server *Server) SetDebug(d bool) {
  server.debug = d
}
func NewServer(hostname string, addr string) *Server {
  server := &Server{false, addr, hostname, nil}
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
  postbox  *postbox.Postbox
  mailbox string
}
func NewSession(
  server *Server, conn net.Conn,
  reader *bufio.Reader, writer *bufio.Writer,
) *Session {
  s := &Session{server, conn, reader, writer, NOT_AUTHENTICATED, "", nil, ""}
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
  mbs, err := sess.postbox.GetMailboxes()
  if err != nil { return err }
  for _, mb := range mbs {
    // fmt.Printf("mb: %+v\n", mb)
    sess.Sendf(
      "* LIST () \"%s\" %s\r\n",
      HIERARCHY_DELIMITER, go_imap.Quote(mb.Name, false),
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
  mb, e := sess.postbox.NewMailbox(mailbox)
  if e != nil {
    sess.Sendf("%s NO Failed to create mailbox\r\n", comm.Tag)
    return e
  } else {
    sess.Sendf("%s OK Created mailbox '%s'\r\n", comm.Tag, mb.Name)
  }
  return nil
}
func (sess *Session) SELECT(comm *Command) error {
  mailbox := strings.TrimSpace(comm.Arguments)
  mailbox = unquote(mailbox)
  // if ok != true { return fmt.Errorf("Error SELECTing mailbox %q", mailbox) }
  
  fmt.Printf("SELECT: %q\n", mailbox)
  sess.Sendf("* FLAGS ()\r\n")
  sess.Sendf("* 0 EXISTS\r\n")
  sess.Sendf("* 0 RECENT\r\n")
  uid, err := sess.postbox.GetUID()
  if err != nil { panic(err) }
  sess.Sendf("* OK [UIDNEXT %s] Next UID\r\n", uid)
  sess.Sendf("%s OK [READ_WRITE] SELECT\r\n", comm.Tag)
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
  if strings.ToUpper(mailbox) == "INBOX" {
    sess.Sendf("* LIST () \"%s\" \"INBOX\"\r\n", HIERARCHY_DELIMITER)
    sess.Sendf("%s OK LIST\r\n", comm.Tag)
  } else {
    mb, err := sess.postbox.GetMailbox(mailbox)
    if err != nil { return err }
    if mb != nil {
      sess.Sendf(
        "* LIST () \"%s\" %s\r\n",
        HIERARCHY_DELIMITER, go_imap.Quote(mb.Name, false),
      )
      sess.Sendf("%s OK LIST\r\n", comm.Tag)
    } else {
      sess.Sendf("%s NO\r\n", comm.Tag);
    }
  }
  return nil
}

func unquote(s string) string {
  if go_imap.Quoted(s) {
    r, ok := go_imap.Unquote(s)
    if ok {
      return r
    } else { return s }
  }
  return s
}

func (sess *Session) LOGIN(comm *Command) error {
  pair := strings.SplitN(comm.Arguments, " ", 2)
  if pair == nil || len(pair) != 2 {
    return fmt.Errorf("Invalid LOGIN arguments: %q", comm.Arguments)
  }
  user := unquote(pair[0])
  // pass := pair[1]
  // TODO: User authentication
  
  sess.username = user
  // TODO: Fix paths
  if sess.postbox == nil {
    // TODO: Make this maybe close postboxes
    sess.postbox = postbox.GetPostbox(
      path.Join("/dirk/projects/courier/test", sess.username),
    )
  }
  
  
  sess.state = AUTHENTICATED
  sess.Sendf("%s OK LOGIN\r\n", comm.Tag)
  fmt.Printf("Logged in: %q\n", user)
  return nil
}

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