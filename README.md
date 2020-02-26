#### MOA Server

#### 简介
    * 基于zk做地址发现
    * 基于turbo协议
    * 使用json序列化协议作为传用户协议传输满足良好跨语言兼容性
    * 基于GroupId划分同服务下的服务，满足初步的治理需要

#### 使用样例
   [样例参考](https://github.com/blackbeans/go-moa-demo)

#### MOA协议简介

   * 基于turbo的二进制协议(https://github.com/blackbeans/turbo/packet)

   * {"action":"/service/bibi/go-moa","params":{"m":"setName","args":["a"]}}
      
      action:理解为服务名称（service-uri）
      
      m:需要调用该服务的方法名称(已经做了go和java关于方法名首字母大写兼容)
      
      args：m方法的调用参数序列。


#### 安装：
    
   安装ZooKeeper
    $Zookeeper/bin/zkServer.sh start
    
    ```
        go get  github.com/blackbeans/go-moa/core
      
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
                app.DestroyApplication()
            }
    
        ```

   * 说明
        - Service为一个服务单元，对应了本服务对外的服务名称、以及对应的接口
    
        - Applcation需要对应的Moa的配置文件，toml类型，具体配置参见./conf/cluster_test. toml
   * 发布服务成功可以使用客户端进行测试，具体[客户端的使用请参考](http://github.com/blackbeans/go-moa-client/blob/master/README.md)

#### Moa状态接口
  
* MOAHOME

   URL : 
    ```http
        http://host:${moaport+1000}/debug/moa
    ```
    返回 :
    
        ```text
        /debug/moa/

        Types of moaprofiles available:
        moa
        index	
        MOA首页
        
        stat	
        MOA系统状态指标
        
        list.clients	
        MOA当前所有连接
        
        list.services	
        MOA发布的服务列表
        
        list.methods	
        MOA来源调用统计信息
        ```
        
* 查询MOA状态信息 
    
    URL : 
    ```http
        http://host:${moaport+1000}/debug/moa/stat
    ```
    返回 :
    
        ```json
        {
            recv: 0, //接收请求数
            proc: 0, //处理请求数
            error: 0, //处理失败数量
            timeout: 0, //超时数量
            invoke_gos: 0, //方法执行的gopool 
            conns: 0, //客户端连接数
            total_gos: 12 //总goroutine数量
         }
        ```
* 查询MOA发布的服务列表

    URL :
     
    ```http
        http://host:${moaport+1000}/debug/moa/list/services
    ```   
    返回 :
    
    ```json
        [
          "/service/go-moa"
        ]

    ```  
   
* 查询服务方法调用统计情况
    
    URL :  
    
    ```http
        http://host:${moaport+1000}/debug/moa/list/methods?service=/service/go-moa
    ```
    返回 :
    
    ```json
        [
            {
              client: "192.168.50.88:63072",
              service_name: "/service/go-moa",
              methods: [
                  {
                    name: "SetName",
                    count: 58939 //总调用次数
                  }
              ]
          }
        ]      
    ```