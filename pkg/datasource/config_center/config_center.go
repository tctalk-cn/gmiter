package config_center

import (
	"git.garena.com/honggang.liu/seamiter-go/ext/datasource"
	sdk "git.garena.com/shopee/platform/config-sdk-go"
	"github.com/pkg/errors"
)

var (
	ErrEmptyKey   = errors.New("property key is empty")
	ErrMissConfig = errors.New("miss config")
)

type Option func(o *options)

type options struct {
	handlers []datasource.PropertyHandler
	//logger   log.LoggerInterface
}

// WithPropertyHandlers 设置属性处理器
func WithPropertyHandlers(handlers ...datasource.PropertyHandler) Option {
	return func(o *options) {
		o.handlers = handlers
	}
}

//// WithLogger 设置Apollo日志
//func WithLogger(logger log.LoggerInterface) Option {
//	return func(o *options) {
//		o.logger = logger
//	}
//}

// apolloDatasource 实现datasource.Datasource
type apolloDatasource struct {
	datasource.Base
	client      *sdk.Client
	propertyKey string
}

// NewDatasource 创建 apollo 数据源
func NewDatasource(propertyKey string, opts ...Option) (datasource.DataSource, error) {
	//if conf == nil {
	//	return nil, ErrMissConfig
	//}
	//if propertyKey == "" {
	//	return nil, ErrEmptyKey
	//}
	//option := &options{
	//	logger: &log.DefaultLogger{},
	//}
	//for _, opt := range opts {
	//	opt(option)
	//}
	//sdk.Client()
	//sdk.Connect(context.Background())
	//apolloClient, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
	//	return conf, nil
	//})
	//if err != nil {
	//	return nil, err
	//}
	//ds := &apolloDatasource{
	//	client:      apolloClient,
	//	propertyKey: propertyKey,
	//}
	//for _, handler := range option.handlers {
	//	ds.AddPropertyHandler(handler)
	//}
	//return ds, nil
	return nil, nil
}

func (a *apolloDatasource) ReadSource() ([]byte, error) {
	//value := a.client.GetValue(a.propertyKey)
	//return []byte(value), nil
	return nil, nil
}

func (a *apolloDatasource) Initialize() error {
	//source, err := a.ReadSource()
	//if err != nil {
	//	return err
	//}
	//a.handle(source)
	//listener := &customChangeListener{
	//	ds: a,
	//}
	//a.client.AddChangeListener(listener)
	return nil
}

func (a *apolloDatasource) Write(bytes []byte) error {
	//log.Warn("apolloDatasource not support syn data to apollo server")
	return nil
}

func (a *apolloDatasource) Close() error {
	return nil
}

func (a *apolloDatasource) handle(source []byte) {
	//err := a.Handle(source)
	//if err != nil {
	//	log.Errorf("update config err: %s", err.Error())
	//}
}
