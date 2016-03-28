#### MOA Server使用方式

* 安装：
    - 因为使用了私有仓库因此必须使用 —insecure参数
    
    ```
    go get -insecure github.com/blackbeans/go-moa/core
    go get -insecure github.com/blackbeans/go-moa/proxy
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
        
        func (self GoMoaDemo) GetDemoName(serviceUri, proto string) (DemoResult, error)      {
            return DemoResult{[]string{"fuck gfw"}, serviceUri}, nil
        }

    ```

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

    - Applcation需要对应的Moa的配置文件，toml类型，具体配置参见./conf/cluster_test.toml
* 发布服务成功可以使用客户端进行测试，具体[客户端的使用请参考](github.com/blackbeans/go-moa-client/blob/master/README.md)

