package main

import (
  "io"
  "os"
  "fmt"
  "time"
  "flag"
  "bufio"
  "strings"

  "golang.org/x/net/context"
  "google.golang.org/grpc"
  "google.golang.org/grpc/credentials"
  "google.golang.org/grpc/grpclog"

  pb "github.com/infiman/http_vs_grpc_test/grpc_test/chat_schema"
)

var (
  tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
  caFile             = flag.String("ca_file", "testdata/ca.pem", "The file containning the CA root cert file")
  serverAddr         = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
  serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")

  userName = "some"
  roomName = ""
)

var currentRoom *pb.Chat_ChatClient

func getRooms(client pb.ChatClient, roomSearch *pb.RoomSearch) error {
  out, err := client.GetRooms(context.Background(), roomSearch)

  if err != nil {
    return err
  }

  for {
    room, err := out.Recv()

    if err == io.EOF {
      break
    }

    if err != nil {
      grpclog.Fatalf("%v.GetRooms(_) = _, %v", client, err)

      return err
    }

    grpclog.Println(room.Name)
  }

  return nil
}

func login(client pb.ChatClient, room *pb.RoomRequest) error {
  out, err := client.Login(context.Background(), room)

  if err != nil {
    grpclog.Fatalf("%v.Login(_) = _, %v", client, err)

    return err
  }

  roomName = room.Name
  createChatConnection(client, roomName)

  grpclog.Println(out.Message)

  return nil
}

func createChatConnection(client pb.ChatClient, roomName string) error {
  room, err := client.Chat(context.Background())
  currentRoom = &room

  if err != nil {
    return err
  }

  grpclog.Println(fmt.Sprintf("Connection with '%v' is established! :)", roomName))

  go func() {
    for {
      if currentRoom != nil {
        message, err := (*currentRoom).Recv()

        if err != nil {
          grpclog.Fatalf(err.Error())
        } else {
          fmt.Println(fmt.Sprintf("MESSAGE [%v]: %v: %v", message.Timestamp, message.UserName, message.Message))
        }
      }
    }
  }()

  if err := chat(fmt.Sprintf("I've joined this room. :)")); err != nil {
    return err
  }

  return nil
}

func chat(message string) error {
  if currentRoom != nil {
    err := (*currentRoom).Send(&pb.Message{
      RoomName: roomName,
      UserName: userName,
      Message: message,
      Timestamp: time.Now().Format(time.RFC850),
    })

    if err != nil {
      return err
    }
  }

  return nil
}

func main() {
  flag.Parse()

  var opts []grpc.DialOption

  if *tls {
    var sn string

    if *serverHostOverride != "" {
      sn = *serverHostOverride
    }

    var creds credentials.TransportCredentials

    if *caFile != "" {
      var err error
      creds, err = credentials.NewClientTLSFromFile(*caFile, sn)

      if err != nil {
        grpclog.Fatalf("Failed to create TLS credentials %v", err)
      }
    } else {
      creds = credentials.NewClientTLSFromCert(nil, sn)
    }

    opts = append(opts, grpc.WithTransportCredentials(creds))
  } else {
    opts = append(opts, grpc.WithInsecure())
  }

  conn, err := grpc.Dial(*serverAddr, opts...)

  if err != nil {
    grpclog.Fatalf("fail to dial: %v", err)
  }

  defer conn.Close()

   client := pb.NewChatClient(conn)

  reader := bufio.NewReader(os.Stdin)
  fmt.Println("Simple Chat")
  fmt.Println("---------------------")

  for {
    text, _ := reader.ReadString('\n')
    text = strings.Replace(text, "\n", "", -1)
    command := strings.SplitN(text, " ", 2)

    switch command[0] {
    case "get_rooms":
      subString := ""

      if len(command) == 2 {
        subString = command[1]
      }

      getRooms(client, &pb.RoomSearch{ SubString: subString })
    case "login":
      roomName := ""

      if len(command) == 2 {
        roomName = command[1]
      }

      login(client, &pb.RoomRequest{ Name: roomName, UserName: userName })
    case "username":
      if len(command) < 2 || len(strings.TrimSpace(command[1])) == 0 {
        grpclog.Println("You havent provided username :(")
      } else {
        userName = command[1]

        grpclog.Println(fmt.Sprintf("You are now registered as '%v'", userName))
      }
    default:
      chat(text)
    }

    if strings.Compare("hi", text) == 0 {
      fmt.Println("hello, Yourself")
    }
  }
}
