# 实现Go Netcat的TCP传输超时处理

在上一课我们已经学习了如何实现超时处理中的TCP连接超时处理，在今天这节课里我们将学习如何处理已建立的读写网络数据的超时。

## 小节目标与实现方法

**小节目标**

1. 为什么需要处理网络传输超时
2. 如何使用Deadline设置读/写超时
3. 如何简单地刷新读写超时Deadline

**实现方法**

通过`SetDeadline(t time.Time) error`、`SetReadDeadline(t time.Time) error`以及`SetWriteDeadline(t time.Time) error`控制网络读写的超时，当前时间加上超时时间就是我们要设置的Deadline。

## 为什么需要处理网络传输超时

相较建立连接的超时而言，建立连接后读写的超时往往更为常见，因为TCP的模型对通信双方而言是一个可以读写的数据流，就像我们读写普通的文件一样。建立连接后连接中并非时时刻刻都在交换数据，一方迟迟接收不到另一方的数据的场景非常常见。一个看起来并不直观的情况是，在没有NAT等记录中间状态的节点存在以及没有设置TCP Keep-Alive的情况下，一条TCP连接可以在没有任何数据交换的情况下一直存在。例如，在我们建立连接后，如果不设置TCP的Keep-Alive，通信的两端可能一直收不到对方的数据而且TCP连接也没被关闭，这就导致对TCP连接的读操作(`conn.Read()`)可以阻塞几十分钟、几个小时甚至几天，直至收到数据或者网络连接被某另一方关闭，这使得我们需要处理对TCP连接的读写超时。

![idle connection](https://img.aeof.top/idle%20connection.png)

<center>客户端连接服务器后迟迟不发送数据</center>

如上图，对于服务器而言，其服务客户端需要消耗内存、CPU等资源，如果客户端连接上服务器后迟迟不发送网络数据，白白占用一条TCP连接，那么服务器需要尽早关闭这条连接以回收资源。同理，如果一个客户端建立连接后并未发送任何数据便静默关闭了这条连接（没有发送FIN、RST包告知服务器关闭连接），那么服务器的`Read()`也会一直阻塞。因此，服务器处理客户端的连接时需要有超时机制，使其尽早关闭无活动的TCP连接，否则我们可以想象一下攻击者可以大量建立与服务器的TCP连接消耗服务器资源，这被称为拒绝服务攻击(DoS, Denial-of-service attack)。我们还可以用客户端探活来防范这种攻击，对这个话题感兴趣的小伙伴可以了解一下TCP Keep-Alive[1]机制以及常见的应用系统心跳的设计。

同理，作为客户端，网络读写中的超时处理也很重要。例如一个本地的聊天软件，在连接到服务器后如果发送数据超过10S，我们就可以提示发送失败，提示用户网络状态不佳并尝试在10S后重连重发，而不是一直等待接收数据。当一个浏览器请求访问某个被墙的站点时，如果Read阻塞时间超时时间还没有获得数据，那么它可以展现一个404页面提示用户无法访问，而不是一直等待。

## 如何用已学的知识实现读写超时控制

Go为我们提供了读写超时控制的API，但在学习新的API前，我们先思考一下，用已学的知识能否实现读写超时控制呢？我们可以使用`net.Conn`的`Close() error`方法关闭一个连接，回收其资源，连接关闭后后续对连接的读写操作都会返回错误。这样，我们可以在读写前设置一个定时器自动关闭该连接，如果我们调用`Read`或者`Write`超时连接就会被关闭，`Read`或者`Write`会立即返回一个`use of closed network connection`的错误；当`Read`/`Write`调用返回时`err`为`nil`则读写未超时，我们可以取消关闭连接：

```go
package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// 与本地的NC TCP服务器建立连接
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	// 设置一个定时函数，三秒后关闭连接
	timer := time.AfterFunc(3*time.Second, func() {
		conn.Close()
	})

	// 读网络数据，三秒后Read还未成功，连接就会被关闭
	// 如果数据读取超时，Read就会返回use of closed network connection的错误
	// 如果数据读取成功，我们就关闭定时器，不关闭这个连接
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalln(err)
	}
	timer.Stop()
	fmt.Printf("读取到: %q\n", string(buf[:n]))
}
```

以上的代码是一个客户端设置3秒超时读写的例子，其连接到运行于`localhost:8080`的TCP服务器，并读网络数据。我们可以在本地运行`nc -l localhost 8080`产生一个TCP的Echo服务器，如果我们不在NC中输入网络数据发送给客户端，那么三秒后客户端的`Read`会因为超时返回一个错误；如果我们在三秒内发送给客户端数据，那么客户端的`Read`会读取成功并返回`err == nil`，客户端会关闭定时器，并打印读到的数据。

## 通过Deadline实现读写超时控制

我们可以看到，上面使用`Close()`的方案可以实现读写的超时控制，但是这种实现存在很大的问题：

1. 关闭一个连接之后连接就无法使用了，需要重新建立连接才能继续读写
2. 依赖于我们在系统的标准库之外关闭连接和取消定时器，因此超时时间精度小

其实，我们可以使用标准库给我们提供的`net.Conn`接口的`SetDeadline(t time.Time) error`、`SetReadDeadline(t time.Time) error`以及`SetWriteDeadline(t time.Time) error`方法，他们分别用于设置读和写的超时时刻、单独设置读的超时时刻以及单独设置写的超时时刻，其中`setDeadline(t)`相当于同时调用`setReadDeadline(t)`以及`setWriteDeadline(t)`。以设置读超时的时刻为例，`setReadDeadline(t)`表示在`t`**时刻(不是t时间段后)**后所有`Read`调用都会返回超时的error，同时正在阻塞的读操作也会返回一个超时的error，而不是继续阻塞。

使用新学的API，我们可以改造一下上述的客户端控制读网络超时的例子，注意API设置的是超时的时刻，其实是**当前时刻+超时时间**：

```go
package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// 与本地的NC TCP服务器建立连接
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

    // 设置读的超时时间
	deadline := time.Now().Add(3 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		log.Fatalln(err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	// 如果超过deadline会返回一个I/O timeout的error
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("读取到: %q\n", string(buf[:n]))
}
```

**执行同样的测试，我们会得到相同的效果，这三个方法调用相较于用`Close()`方法关闭连接实现超时的方法，具有以下的优点：**

1. Deadline是在连接对象上应用层设置的超时时间，Deadline超时后并不影响传输层连接的状态，只是设置了Go代码中`net.Conn`的截止状态。也就是说超时之后TCP连接并不会因此而关闭，在一次读写超过Deadline错误返回后，我们可以继续调整连接的Deadline到当前时间之后，继续使用这条连接进行读写
2. 可以单独设置读或者写的截止时间
3. 由标准库实现，截止时间的精度较高

## 怎么封装超时读写操作

我们可以通过调用Deadline的三个方法来控制超时，但是每次读写都需要重复设置读写截止时刻=当前时间+超时时间，其实并不方便。因为很多时候我们的读写超时时间往往固定，我们可以封装一个具有读写超时的网络连接，在其上读写更为方便：

```go
// 一个对net.Conn的包装类型
// 如果设置了读/写的Deadline，那么读/写的超时时间由其Deadline决定。
// 如果没有设置读/写的Deadline，那么每一次调用读/写的超时时刻=当前时间+超时时长
type timeoutConn struct {
    // 嵌入一个连接，可以不用实现net.Conn部分方法，直接用这个连接的
	net.Conn
    // 标记用户是否设置了读的截止时间
	isReadDeadlineSet bool
    // 标记用户是否设置了写的截止时间
	isWriteDeadlineSet bool
	ReadTimeout time.Duration
	WriteTimeout time.Duration
}

func newTimeoutConn(conn net.Conn, readTimeout, writeTimeout time.Duration) net.Conn {
	return &timeoutConn{
		Conn:         conn,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}

// 实现net.Conn接口的Read方法，如果设置了读的Deadline就按照读的Deadline，否则按照设置的timeout
func (tr *timeoutConn) Read(p []byte) (n int, err error) {
	if !tr.isReadDeadlineSet && tr.ReadTimeout != 0 {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.ReadTimeout))
	}
	return tr.Conn.Read(p)
}

// 实现net.Conn接口的Write方法，如果设置了写的Deadline就按照读的Deadline，否则按照设置的timeout
func (tr *timeoutConn) Write(p []byte) (n int, err error) {
	if !tr.isWriteDeadlineSet && tr.WriteTimeout != 0 {
		tr.SetWriteDeadline(time.Now().Add(tr.WriteTimeout))
	}
	return tr.Conn.Write(p)
}

func (tr *timeoutConn) SetDeadline(t time.Time) error {
	if err := tr.SetReadDeadline(t); err != nil {
		return err
	}
	if err := tr.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (tr *timeoutConn) SetReadDeadline(t time.Time) error {
	var zero time.Time
	if t == zero {
		tr.isReadDeadlineSet = false
	} else {
		tr.isReadDeadlineSet = true
	}
	return tr.Conn.SetReadDeadline(t)
}

func (tr *timeoutConn) SetWriteDeadline(t time.Time) error {
	var zero time.Time
	if t == zero {
		tr.isWriteDeadlineSet = false
	} else {
		tr.isWriteDeadlineSet = true
	}
	return tr.Conn.SetWriteDeadline(t)
}
```

#### 为什么要嵌入一个字段

我们在这里把一个`net.Conn`嵌入为`timeoutConn`类型的一个字段，因为我们为了通用性要让这个`*timeoutConn`类型实现`net.Conn`接口，这样我们就需要为`*timeoutConn`实现`net.Conn`的所有方法。而我们又不想要重新编写一部分方法的代码，比如说`LocalAddr()`，这个时候我们嵌入这个我们实际操作的`net.Conn`类型的网络连接，就可以直接调用它的方法了：

```go
conn, _ := net.Dial("tcp", "www.google.com:http")
defer conn.Close()

// 产生一个包装的有读写超时的net.Conn接口变量，实际是timeoutConn类型
conn = newTimeoutConn(conn, 10 * time.Second, 3 * time.Second)

// 实际调用的是conn里存放的net.Conn类型的连接的LocalAddr()
fmt.Println(conn.LocalAddr())
```

#### 修改后的代码

```go
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

var (
	verbose       bool
	timeoutSecond int
)

const DefaultTimeout = 0

func init() {
	flag.IntVar(&timeoutSecond, "w", DefaultTimeout, "Connections which cannot be established or are idle timeout after timeout seconds.")
	flag.BoolVar(&verbose, "v", false, "Produce more verbose output.")
	flag.Parse()
}

func checkError(err error) {
	if err == nil {
		return
	}

	// only output error when verbose mode is on
	if verbose {
		fmt.Fprint(os.Stderr, err)
	}
	os.Exit(1)
}

func main() {
	// parse arguments
	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	host := args[0]
	port := args[1]

	// connect to the server
	timeout := time.Duration(timeoutSecond) * time.Second
	conn, err := net.DialTimeout("tcp", host+":"+port, timeout)
	checkError(err)
	defer conn.Close()
	if verbose {
		fmt.Printf("Succeeded to connect to %s %s port!\n", host, port)
	}

	conn = NewTimeoutConn(conn, timeout, timeout)
	go func() {
		io.Copy(conn, os.Stdin)
	}()
	_, err = io.Copy(os.Stdout, conn)
	checkError(err)
}
```

我们可以运行`go run main.go timeoutConn.go -v -w 3 baidu.com http`连接到`baidu.com`，如果我们不发送数据，那么连接的`Read`会在3秒后超时。当然，这样运行比较麻烦，我们可以在项目根目录运行`go install`安装该文件到`GOPATH`环境变量的`bin`目录下，我们可以在这个目录下找到`gonc`并运行，如果我们把这个目录加到了`PATH`还可以直接在其他目录外运行`gonc`:

```shell
go install
gonc -v -w 3 baidu.com http
```

## 思考

如何用单元测试测试我们封装的这个类型是否实现有问题呢？

## 参考文献

[1] *《TCP/IP详解，卷1：协议》，第17章TCP Keepalive*