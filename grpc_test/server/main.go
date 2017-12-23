package main

import (
  "net"
  "fmt"
  "flag"
  "strings"

  "golang.org/x/net/context"
  "google.golang.org/grpc"
  "google.golang.org/grpc/credentials"
  "google.golang.org/grpc/grpclog"

  pb "github.com/infiman/http_vs_grpc_test/grpc_test/chat_schema"
)

type ChatRoom struct {
  room *pb.Room
  connections map[string]*pb.Chat_ChatServer
}

var (
  tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
  certFile   = flag.String("cert_file", "testdata/server1.pem", "The TLS cert file")
  keyFile    = flag.String("key_file", "testdata/server1.key", "The TLS key file")
  jsonDBFile = flag.String("json_db_file", "testdata/route_guide_db.json", "A json file containing a list of features")
  port       = flag.Int("port", 10000, "The server port")
)

type ChatServer struct {
  chatRooms []*ChatRoom
}

func (chatServ *ChatServer) GetRooms(in *pb.RoomSearch, out pb.Chat_GetRoomsServer) error {
  for _, chatRoom := range chatServ.chatRooms {
    if strings.Contains(chatRoom.room.Name, in.SubString) {
      if err := out.Send(chatRoom.room); err != nil {
        return err
      }
    }
  }

  return nil
}

func (chatServ *ChatServer) Login(_ context.Context, in *pb.RoomRequest) (*pb.AuthResponse, error) {
  for _, chatRoom := range chatServ.chatRooms {
    if strings.Compare(chatRoom.room.Name, in.Name) == 0 {
      chatRoom.connections[in.UserName] = nil

      return &pb.AuthResponse{ Status: 201, Message: "You have been logged in :)" }, nil
    }
  }

  return &pb.AuthResponse{ Status: 404, Message: "Room has not been found" }, nil
}

func (chatServ *ChatServer) Chat(stream pb.Chat_ChatServer) error {
  for {
    message, err := stream.Recv()

    if err != nil {
      return err
    }

    for _, chatRoom := range chatServ.chatRooms {
      if strings.Compare(chatRoom.room.Name, message.RoomName) == 0 {
        if chatRoom.connections[message.UserName] == nil {
          chatRoom.connections[message.UserName] = &stream
        }

        for _, conn := range chatRoom.connections {
          if conn != nil {
            (*conn).Send(message)
          }
        }
      }
    }

    if err != nil {
      grpclog.Fatalf(err.Error())

      return err
    }
  }

  return nil
}

func (chatServ *ChatServer) Logout(_ context.Context, in *pb.RoomRequest) (*pb.AuthResponse, error) {
  return nil, nil
}

func initChatServer() *ChatServer {
  server := new(ChatServer)

  for i := 0; i < 7; i++ {
    server.chatRooms = append(
      server.chatRooms,
      &ChatRoom{
        room: &pb.Room{
          Name: fmt.Sprintf("Room %v", i),
          UsersCount: 0,
        },
        connections: make(map[string]*pb.Chat_ChatServer),
      },
    )
  }

  return server
}

func main() {
  flag.Parse()

  lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))

  if err != nil {
    grpclog.Fatalf("failed to listen: %v", err)
  }

  var opts []grpc.ServerOption

  if *tls {
    creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)

    if err != nil {
      grpclog.Fatalf("Failed to generate credentials %v", err)
    }

    opts = []grpc.ServerOption{ grpc.Creds(creds) }
  }

  grpcServer := grpc.NewServer(opts...)

  pb.RegisterChatServer(grpcServer, initChatServer())
  grpcServer.Serve(lis)
}
