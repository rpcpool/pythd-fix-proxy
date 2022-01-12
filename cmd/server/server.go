package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"

	fix42mdr "github.com/quickfixgo/fix42/marketdatarequest"

	fix42er "github.com/quickfixgo/fix42/executionreport"
	"github.com/quickfixgo/quickfix"
)

type Application struct {
	orderID int
	execID  int
	setting *quickfix.SessionSettings
	*quickfix.MessageRouter
}

func newApp() *Application {
	app := Application{
		MessageRouter: quickfix.NewMessageRouter(),
	}
	app.AddRoute(fix42er.Route(app.OnFIX42ExecutionReport))

	return &app
}

func (a *Application) OnFIX42ExecutionReport(msg fix42er.ExecutionReport, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf("\n ===========OnFIX42ExecutionReport========== \n")
	fmt.Printf("%+v", msg)
	fmt.Printf("\n ===================== \n")
	return nil
}

//Notification of a session begin created.
func (a *Application) OnCreate(sessionID quickfix.SessionID) {
}

//Notification of a session successfully logging on.
func (a *Application) OnLogon(sessionID quickfix.SessionID) {
}

//Notification of a session logging off or disconnecting.
func (a *Application) OnLogout(sessionID quickfix.SessionID) {
}

//Notification of admin message being sent to target.
func (a *Application) ToAdmin(message *quickfix.Message, sessionID quickfix.SessionID) {}

//Notification of app message being sent to target.
func (a *Application) ToApp(message *quickfix.Message, sessionID quickfix.SessionID) error {
	return nil
}

//Notification of admin message being received from target.
func (a *Application) FromAdmin(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	return nil
}

//Notification of app message being received from target.
func (a *Application) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {

	return a.Route(message, sessionID)
}

type executor struct {
	*quickfix.MessageRouter
}

type header interface {
	Set(f quickfix.FieldWriter) *quickfix.FieldMap
}

func setHeader(h header) {

}

func targetCompID(v string) field.TargetCompIDField {
	return field.NewTargetCompID(v)
}

func senderCompID(v string) field.SenderCompIDField {
	return field.NewSenderCompID(v)
}

var cfgFileName = flag.String("cfg", "acceptor.cfg", "Acceptor config file")

func main() {
	flag.Parse()
	if err := start(*cfgFileName); err != nil {
		fmt.Println("Err start acceptor ", err)
	}
}

func start(cfgFileName string) error {
	cfg, err := os.Open(cfgFileName)
	if err != nil {
		return fmt.Errorf("Error opening %v, %v\n", cfgFileName, err)
	}
	defer cfg.Close()
	stringData, readErr := ioutil.ReadAll(cfg)
	if readErr != nil {
		return fmt.Errorf("Error reading cfg: %s,", readErr)
	}

	appSettings, err := quickfix.ParseSettings(bytes.NewReader(stringData))
	if err != nil {
		return fmt.Errorf("Error reading cfg: %s,", err)
	}

	logFactory := quickfix.NewScreenLogFactory()
	app := newApp()

	acceptor, err := quickfix.NewAcceptor(app, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
	if err != nil {
		return fmt.Errorf("Unable to create Acceptor: %s\n", err)
	}

	err = acceptor.Start()
	if err != nil {
		return fmt.Errorf("Unable to start Acceptor: %s\n", err)
	}

	global := appSettings.SessionSettings()
	for k, v := range global {
		if k.BeginString == quickfix.BeginStringFIX42 {
			app.setting = v
			time.Sleep(5 * time.Second)
			msg := app.makeFix42MarketDataRequest("BCHUSD")
			err := quickfix.SendToTarget(msg, k)

			if err != nil {
				return fmt.Errorf("Error SendToTarget : %s,", err)
			}
		}
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	acceptor.Stop()

	return nil
}

func (app *Application) makeFix42MarketDataRequest(symbol string) *quickfix.Message {
	fmt.Printf("%+v", app.setting)
	sender, err := app.setting.Setting("SenderCompID")
	if err != nil {
		panic(fmt.Sprintf("Miss SenderCompID %+v", err))
	}
	target, err := app.setting.Setting("TargetCompID")
	if err != nil {
		panic(fmt.Sprintf("Miss SenderCompID %+v", err))
	}

	request := fix42mdr.New(field.NewMDReqID("MARKETDATAID"),
		field.NewSubscriptionRequestType(enum.SubscriptionRequestType_SNAPSHOT),
		field.NewMarketDepth(0),
	)

	entryTypes := fix42mdr.NewNoMDEntryTypesRepeatingGroup()
	entryTypes.Add().SetMDEntryType(enum.MDEntryType_BID)
	request.SetNoMDEntryTypes(entryTypes)

	relatedSym := fix42mdr.NewNoRelatedSymRepeatingGroup()
	relatedSym.Add().SetSymbol(symbol)
	request.SetNoRelatedSym(relatedSym)

	request.Header.Set(senderCompID(sender))
	request.Header.Set(targetCompID(target))

	return request.ToMessage()
}
