# 实现Go Netcat的简单TCP客户端

千里之行始于足下，这一节是Go网络编程实战课的第一讲，我们将学习如何实现我们的Go语言Netcat(gonc)的TCP客户端功能。简单来讲，我们这节课将要实现读取命令行参数并根据得到命令行参数(Arguments)建立TCP连接以及进行基本网络通信的功能。

## 小节目标与实现方法

**小节目标**

1. 如何获取命令行参数，并且进行错误处理
2. 如何使用Go的net包进行TCP连接和数据读写，并释放资源

**实现方法**

1. 通过`os.Args`获得命令行参数切片
2. 通过`net.Dial()`或者`net.DialTCP()`函数建立TCP连接
3. 通过`net.Conn`类型的`Close()`方法回收连接资源
4. 通过连接对象进行TCP的网络读写

## 如何获取命令行参数

在部分程序语言如C、C++和Java中，我们可以通过main函数的参数访问命令行参数列表，而在部分程序语言如Go中，我们可以通过标准库函数或者变量访问命令行参数。在Go中，`os`包下的[`os.Args`](https://pkg.go.dev/os?utm_source=gopls#Args)变量是一个包含命令行参数列表的字符串切片，它在`os`包下的定义是`var Args []string`。其中第0个参数通常是可执行文件的路径，如当我们键入`./program mike 18`运行一个Go程序`program`时，`os.Args`内容则为`[]String{"./program", "mike", "18"}`。

在NC中，目标Host和端口号作为两个参数传递给我们的命令行程序，其中IP既可以是域名也可以是IP，如`nc www.baidu.com 80`或者`nc 192.0.0.6 443`，我们可以通过`os.Args[1]`和`os.Args[2]`得到要访问的Host和端口：

```go
// 确保传递的参数个数正确
// nc www.baidu.com 80
if len(os.Args) != 3 {
    log.Fatal("Usage: nc host port")
}
host := os.Args[1]
port := os.Args[2]
```

## 如何建立TCP连接

通过标准库`net`包，我们可以轻松地建立一个TCP连接，可以利用到的函数通常有两个，分别是`func Dial(network, address string) (Conn, error)`以及`func DialTCP(network string, laddr, raddr *TCPAddr) (*TCPConn, error)`，前者更通用，适用于TCP、UDP、IP以及Unix Domain Socket（后续章节会使用到）的网络，而后者则仅用于建立TCP连接。这里要特别注意的是，虽说`net.Dial()`名为`Dial`，但是仅仅是一个通用的获取`net.Conn`接口用于读写的函数，当作用于UDP类型时，它并不会发起网络IO，例如`net.Dial("udp", "www.baidu.com:10231")`并不会发起三次握手，不会有连接过程。

通常情况下，使用Go的`net`包进行网络通信我们常常使用`net.Conn`接口就足够了，我们可以调用`net.Dial()`获取TCP、UDP以及IP等网络通信，在我们需要进行特定的控制如TCP的Keep Alive时我们才需要特定的实现类型如`*net.TCPConn`（可以通过类型断言获取接口变量的具体类型的值）。因为`net.Dial()`函数简单，而且可以用于UDP、TCP等多种方式的通信，我们通常使用这个函数建立TCP连接，而我们也可以使用`net.DialTCP()`建立TCP连接，这个函数返回的是一个`*net.TCPConn`变量，可以进行更多的控制，但是函数参数构造较为麻烦。

通过`net.Dial("tcp", host+":"+port)`我们将建立TCP连接，这是一个阻塞调用，调用时程序会发起系统调用进行一次TCP三次握手，如图[1]：![img](https://media.geeksforgeeks.org/wp-content/uploads/handshake-1.png)

在处理完网络数据后，我们通常需要回收对应的IO资源，例如在Linux/UNIX中通常需要关闭对应的文件并释放相应的缓冲区。我们需要调用`conn.Close()`方法，其会发送一个FIN包，指示目标端口断开连接，这种资源释放的操作通常通过Go的`defer`在连接建立成功后就进行延迟调用来处理，代码如下：

```go
conn, err := net.Dial("tcp", host+":"+port)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

## 如何读写网络数据

通过调用`net.Dial()`，我们得到了一个[`net.Conn`](https://pkg.go.dev/net#Conn)连接对象，这个接口的定义如下：

```go
type Conn interface {
    // 读数据，io.Reader
    Read(b []byte) (n int, err error)

    // 写数据，io.Writer
    Write(b []byte) (n int, err error)

    // 回收资源
    Close() error

    // 本地地址
    LocalAddr() Addr

    // 远程地址(目标Host:port)地址
    RemoteAddr() Addr

    // 设置网络IO的超时时间
    SetDeadline(t time.Time) error

    // 设置读网络数据的超时时间
    SetReadDeadline(t time.Time) error

    // 设置写网络数据的超时时间
    SetWriteDeadline(t time.Time) error
}
```

我们可以看到，这个接口有`Read()`、`Write()`和`Close()`方法，分别用于读取、发送网络数据和回收IO资源，因此它的具体类型一定实现了标准库的`io.ReadWriteCloser`接口，那我们就可以使用很多标准库函数对其进行操作。

在NC中，我们要读取用户标准输入（通常是我们的键盘，也就是Linux的stdin）作为网络数据发送给目标Host，并将读取到的网络数据写入到进程的标准输出（通常是我们的终端，也就是Linux进程的stdout），而Go中的标准输入是`Stdin`、标准输出是`Stdout`，他们都是`*File`类型，实现了`io.ReadWriter`接口。因此，我们可以用[`func Copy(dst Writer, src Reader) (written int64, err error)`](https://pkg.go.dev/io#Copy)函数不断从一个Reader `src`中读取数据并写入到Writer `dst`中，这样就可以简单地实现网络数据与标准输入输出的拷贝了：

```go
go func() {
    // 不断读取标准输入数据发送给目标端口
    io.Copy(conn, os.Stdin)
}()
// 不断读取目标端口的数据并写入到标准输出（例如打印到屏幕）
io.Copy(os.Stdin, conn)
```

## 思考

如果不使用`io.Copy()`，我们要实现相同的功能需要写多少代码呢？

## 参考文献

[1] [TCP 3-Way Handshake Process - GeeksforGeeks](https://www.geeksforgeeks.org/tcp-3-way-handshake-process/)