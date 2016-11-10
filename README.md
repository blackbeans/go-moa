#### MOA Server

#### 简介
    * 基于zk做地址发现
    * 基于redis的get、ping、info协议构建协议通讯
    * 使用json序列化协议满足良好跨语言兼容性
    * 使用当前众多语言的redisclient即可以完成客户端开发。
    * 基于GroupId划分同服务下的服务，满足初步的治理需要

#### 使用样例
   [样例参考](https://github.com/blackbeans/go-moa-demo)

#### Redis协议简介

   * 可以使用redis-cli -h -p get/ping/info 命令访问服务或发起服务调用

   * Get
   
   exp:

      get {"action":"/service/bibi/go-moa","params":{"m":"setName","args":["a"]}}
      
      action:理解为服务名称（service-uri）
      
      m:需要调用该服务的方法名称(已经做了go和java关于方法名首字母大写兼容)
      
      args：m方法的调用参数序列。

   * PING 
     
       同 Redis的PING返回PONG

   * INFO
   
      同 Redis的INFO，返回MOA和网络状态(json数据moa与network节点)。

      exp:
      
        {"moa":{"recv":0,"proc":0,"error":0},"network":{"read_count":1,"read_bytes":9,"write_count":0,"write_bytes":0,"dispatcher_go":1,"connections":1}}



#### 安装：
    
   安装ZooKeeper
    $Zookeeper/bin/zkServer.sh start
    
    ```
    go get  github.com/blackbeans/go-moa/core
    go get  github.com/blackbeans/go-moa/proxy
    ```
   
   * 定义服务的接口对应
   
   - 例如接口为：

        ```goalng
            //接口
            type DemoResult struct {
                Hosts []string `json:"hosts"`
                Uri   string   `json:"uri"`
            }
            
            type IGoMoaDemo interface {
                GetDemoName(serviceUri, proto string) (DemoResult, error)
            }
            //服务实现
            type GoMoaDemo struct {
            }
            
            func (self GoMoaDemo) GetDemoName(serviceUri, proto string) (   DemoResult, error)      {
                return DemoResult{[]string{"fuck gfw"}, serviceUri}, nil
            }
         ```
   - 约定：
            为了给客户端友好的返回错误信息，go-moa的服务接口最后一个返回必须为error类型。并且为了满足Java单一返回结果所以返回参数最多2个。
            
   * 服务端启动启动：
    
         ```goalng
    
            func main(){
                app := core.NewApplcation("./conf/cluster_test.toml", 
                func() []proxy.Service {
                    return []proxy.Service{
                        proxy.Service{
                            ServiceUri: "/service/bibi/go-moa",
                            Instance:   GoMoaDemo{},
                            Interface:  (*IGoMoaDemo)(nil)}}
                })
            
                //设置启动项
                ch := make(chan os.Signal, 1)
                signal.Notify(ch, os.Kill)
                //kill掉的server
                <-ch
                app.DestoryApplication()
            }
    
        ```

   * 说明
        - Service为一个服务单元，对应了本服务对外的服务名称、以及对应的接口
    
        - Applcation需要对应的Moa的配置文件，toml类型，具体配置参见./conf/cluster_test. toml
   * 发布服务成功可以使用客户端进行测试，具体[客户端的使用请参考](http://github.    com/blackbeans/go-moa-client/blob/master/README.md)

#### Benchmark

    env:Macbook Pro 2.2 GHz Intel Core i7
    
    redis-benchmark result : 53527.46 requests per second

    go test --bench=".*" github.com/blackbeans/go-moa/core -run=BenchmarkApplication

    BenchmarkApplication-8     20000         64517 ns/op

