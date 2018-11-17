# netjumper
From time to time, my go application need to connect to network from DigitalOcean,
but digital ocean sucks when connecting to China, it routes traffic to the US.

However, my home network supports good connection to China, but my home has no long-running servers like digital ocean.

Therefore, I created this network jumper, that run as service, act as a VPN kind of service
allow DigitalOcean go application to connect to china

This is how the protocol works
a) First, the client connect to jump host
b) jump host send back a 32 bytes challenge
c) client must use sha1 to hash the concat(challenge, shared secret), and send to jump host as challenge response
d) server send 0 indicating challenge worked, or 1 indicating challenge not worked (connection to be closed in this case)
e) client send the destination network to connect to in host:port format. e.g. www.baidu.com:443, or 11.133.22.3:444
f) jump server attempts to make the connection, if succeeded, send 0 to client, or 1 indicating failure, connection will be closed
g) after 0 received from client, the client <-> jumphost <-> remote server will be bi-directionally duplex piped. 


This can work in any protocol. An example of such usage

```go
package main

import (
        njlib "github.com/wushilin/netjumper/lib"
        "log"
        "fmt"
)

func main() {
        // creates a http client directly with a jump host, returns a *http.Client
        // If you are interested in Http, you can just do this
        httpClient := njlib.JumperClient("home.myhome.net:9527", "superBigSecret")
        fmt.Println("Http Client is created", httpClient)

        j := &njlib.Jumper{"home.myhome.net:9527", "superBigSecret"}
        // if you want to do TCP, you can use this:
        conn, err := j.Dialer("tcp", "www.google.com:443")
        if err != nil {
                log.Fatal("something is wrong")
        }
        fmt.Println("Conn is established")
        // do something with conn. It is a real connection with www.google.com:443, via the jump host
        defer conn.Close()
}
```
Enjoy!
