# 实现Go Netcat的TCP连接超时处理

在这一节和下一节中，我们将学习如何实现网络编程中的超时，这通常包括TCP连接建立的超时以及网络数据读写的超时，在这一节中我们将学习超时的概念以及如何实现TCP连接建立的超时逻辑。

## 小节目标与实现方法

**小节目标**

1. 了解网络编程中的超时
2. 了解如何实现连接建立的超时

**实现方法**

1. 读取命令行中的flag
2. 解析得到除flag外的参数，当前是目标Host和Port
3. 根据超时flag(`--w`、`-w`)进行连接
4. 根据是否设置详尽模式(verbose mode, `-v`, `--v`)输出信息

## 网络编程中的超时

在网络编程中，当我们使用TCP传输层通信时，主要有两种可能发生超时的情况：

1. TCP三次握手建立连接可能发生超时
2. TCP连接建立后交换数据时可能超时

为什么要处理超时呢？这是因为连接建立（`net.Dial()`、`net.DialTCP()`，函数参数被省略）和网络数据读写（`net.Conn.Read()`、`net.Conn.Write()`）都是阻塞式的调用，当函数返回时要么是TCP连接建立成功，要么是因为参数或网络原因失败。如果建立连接或者发送接收数据时我们迟迟没有收到对方的包，那么我们就必须持续等待，直至操作系统网络协议栈超时，在函数返回前整个Goroutine的后续代码都无法执行。而操作系统网络协议栈的超时时间根据操作系统有所不同，而且根据系统配置不同也会发生变化，这可能是十秒，也可能是一小时甚至一天，给程序带来了不确定性。

试想，你编写了一个实时聊天软件，如果连接到服务器时忘记处理超时，而目标服务器已经宕机或者因为其他原因不可达，那么`net.Dial()`会一直阻塞直到几十秒或者几小时后操作系统网络协议栈超时函数返回，后面的代码都无法执行，整个Goroutine好像卡住了一样，后续的逻辑无法执行。这并不是我们想要的，我们想要的结果是如果5秒钟还无法连接到服务器，我们就报告当前服务器状态不佳或者尝试重连，这样才是我们的理想行为。

## 如何实现TCP连接超时处理

之前我们使用的建立TCP连接的函数是`net.Dial()`以及`net.DialTCP()`，然而当我们需要控制连接过程时，我们就需要使用`net.DialTimeout()`函数或者是通过`net.Dialer`类型变量的方法了。

#### 方案一：使用`net.DialTimeout()`

我们可以使用`net`包下的`net.DialTimeout()`方法实现带有超时处理的TCP连接，它的函数实现如下：

```go
func DialTimeout(network, address string, timeout time.Duration) (Conn, error) {
    d := Dialer{Timeout: timeout}
    return d.Dial(network, address)
}
```

这个函数在`net.Dial()`的参数基础上，增加了一个超时时长的参数，这样我们就可以控制连接时的超时时间了：

```go
conn, err := net.DialTimeout("tcp", host+":"+port, time.Duration(timeout)*time.Second)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

#### 方案二：直接操作`net.Dialer`

在上文中我们看到，`net.DialTimeout()`的实现使用了一个`net.Dialer`类型的变量来实现连接，其实`net.Dial()`函数也是一样的，如果我们需要对连接建立进行进一步控制就必须使用`net.Dialer`类型。`net.DialTimeout()`的实现是配置`net.Dialer`的`timeout`字段，并且调用它的`Dial()`方法建立连接，我们可以查看`net.Dialer`的`Dial()`方法的定义：

```go
func (d *Dialer) Dial(network, address string) (Conn, error) {
    return d.DialContext(context.Background(), network, address)
}
```

可以看到，`net.Dialer`的`Dial()`方法（不是`net.Dial()`函数）其实是对`DialContext()`方法的封装，我们也可以直接调用`DialContext()`方法来实现控制，通过`DialContext()`方法，我们既可以实现超时取消，也可以在上层函数/方法里取消连接。

## 如何解析命令行参数中的flag

不同于上一节中我们的NC只有普通的命令行参数（Argument），在这一节我们需要实现NC的超时时长参数，新的命令行形式是`nc [-w timeout_in_seconds] host port`，这里的`-w timeout_in_seconds`的flag可有可没有，而且随着我们的NC的参数不断增多，它的位置还可能变化，如`nc -u -w 3 www.google.com 80`与`nc -w 3 -u www.google.com 80`的执行效果应该相同。因此，如果不借助其他包的话，我们要判断参数的位置以及是否提供部分参数，这需要增加一些可读性较低的代码，而且是比较麻烦的。幸好Go的标准库有`flag`包，通过`flag`包我们可以将用户输入的flag和程序的变量绑定起来，例如将`-w 3`的数值3绑定到的整型变量`timeout`上，这是如何实现的呢？

我们已经知道`os.Args`切片存储了进程启动的命令行参数，flag作为命令行参数列表的一部分和其他值没有不同，当我们在终端输入`go run main.go -w 3 www.google.com 80`时，`os.Args`的内容为`["/tmp/go-build2447458550/b001/exe/main","-w","3","www.google.com", "80"]`，注意因为第一个参数是Go工具链产生的临时可执行文件，所以名称会有所不同。我们可以使用`flag.IntVar(&timeout, "w", DefaultTimeout, "timeout")`读取`os.Args`内容，找到`"-w"`以及其下一个参数`"3"`，将字符串`"3"`解析为整数后赋值给整型变量`timeout`。随着参数变多，我们可能有很多类似`flag.IntVar()`的调用用于绑定flag值，除此之外我们还需要调用一次`flag.Parse()`用于真正执行命令行参数的解析。通常我们将flag处理的逻辑放在Go源文件的一个特殊函数`init()`中，这个函数会在`main`函数执行前执行，用于一些资源的初始化，代码如下：

```go
var (
    // 详尽模式是否打开
	verbose bool
    // 超时时间有多少秒
	timeout int
)

const DefaultTimeout = 0

func init() {
	flag.IntVar(&timeout, "w", DefaultTimeout, "Connections which cannot be established or are idle timeout after timeout seconds.")
	flag.BoolVar(&verbose, "v", false, "Produce more verbose output.")
	flag.Parse()
}
```

那么剩余的`Host`和`Port`参数呢？虽然`os.Args`变量是可以修改的，但是`flag`包下的函数不会直接修改`os.Args`的内容，因此`os.Args`内容依然为`["/tmp/go-build2447458550/b001/exe/main","-w","3","www.google.com", "80"]`，那么我们如何获取Host参数呢？我们可以直接用`host := os.Args[3]`得到Host吗？这是不可以的，我们怎么能保证用户输入的Host就一定是3号元素呢，如果不提供`-w`参数它就成了1号元素了呀，如`["/tmp/go-build2447458550/b001/exe/main", "www.google.com", "80"]`，所以这种方法是不可取的。在这里，我们可以**使用`flag.Args()`函数获取调用`flag.Parse()`去除掉识别到的flag之后的所有命令行参数**，如执行`["/tmp/go-build2447458550/b001/exe/main","-w","3","www.google.com", "80"]`时，调用完`flag.Parse()`之后，`flag.Args()`返回的内容就是`["www.google.com" "80"]`，注意这里的参数不包含原来`os.Args`的零号参数。

# 思考

如何通过`net.Dialer`的`DialContext()`方法，同时连接`www.google.com:80`、`www.baidu.com:80`、`www.youtube.com:80`，如果其中一条TCP连接成功则取消其余的连接。

## 参考文献

[1] [flag package - flag - Go Packages](https://pkg.go.dev/flag)

[2] [Go by Example: Command-Line Flags](https://gobyexample.com/command-line-flags)