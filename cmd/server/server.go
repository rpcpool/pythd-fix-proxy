package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"

	fix42lo "github.com/quickfixgo/fix42/logon"
	fix42mdr "github.com/quickfixgo/fix42/marketdatarequest"
	fix42sd "github.com/quickfixgo/fix42/securitydefinition"
	fix42sdr "github.com/quickfixgo/fix42/securitydefinitionrequest"

	fix42er "github.com/quickfixgo/fix42/executionreport"
	"github.com/quickfixgo/quickfix"
)

type Application struct {
	orderID int
	execID  int
	symbols map[string]string
	mu      sync.Mutex
	setting *quickfix.SessionSettings
	*quickfix.MessageRouter
}

func (e *Application) genExecID() field.SecurityReqIDField {
	e.execID++
	return field.NewSecurityReqID(strconv.Itoa(e.execID))
}

func newApp() *Application {
	app := Application{
		MessageRouter: quickfix.NewMessageRouter(),
		symbols:       make(map[string]string),
	}
	app.AddRoute(fix42er.Route(app.OnFIX42ExecutionReport))
	app.AddRoute(fix42sd.Route(app.OnFIX42SecurityDefinition))

	return &app
}

func (a *Application) OnFIX42SecurityDefinition(msg fix42sd.SecurityDefinition, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	symbol, err := msg.GetSymbol()
	if err != nil {
		return err
	}

	{
		a.mu.Lock()
		a.symbols[symbol] = symbol
		defer a.mu.Unlock()
	}
	return nil
}

func (a *Application) OnFIX42ExecutionReport(msg fix42er.ExecutionReport, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf("\n ===========OnFIX42ExecutionReport========== \n")
	fmt.Printf("%+v", msg)
	fmt.Printf("\n ===================== \n")
	return nil
}

//Notification of a session begin created.
func (a *Application) OnCreate(sessionID quickfix.SessionID) {
	fmt.Println("OnCreate")
}

func (a *Application) OnLogon(sessionID quickfix.SessionID) {
	fmt.Println("OnLogon")
	msg := fix42sdr.New(a.genExecID(), field.NewSecurityRequestType(enum.SecurityRequestType_SYMBOL))
	err := quickfix.SendToTarget(msg, sessionID)
	if err != nil {
		fmt.Printf("Error SendToTarget : %s,", err)
	} else {
		fmt.Printf("\nSend ok %+v \n", msg)
	}
	go func() {
		time.Sleep(10 * time.Second)
		for {
			time.Sleep(10 * time.Second)
			for _, s := range a.symbols {
				msg := a.makeFix42MarketDataRequest(s)
				err := quickfix.SendToTarget(msg, sessionID)
				fmt.Printf("Send makeFix42MarketDataRequest %+v \n", msg)
				if err != nil {
					fmt.Printf("Error SendToTarget : %s,", err)
				} else {
					fmt.Printf("\nSend ok %+v \n", msg)
				}
			}
		}
	}()
}

//Notification of a session logging off or disconnecting.
func (a *Application) OnLogout(sessionID quickfix.SessionID) {
	fmt.Println("OnLogout")
}

//Notification of admin message being sent to target.
func (a *Application) ToAdmin(message *quickfix.Message, sessionID quickfix.SessionID) {
	password, err := a.setting.Setting("Password")
	if err != nil {
		panic(fmt.Sprintf("Miss SenderCompID %+v", err))
	}

	message.Header.SetString(quickfix.Tag(554), password)
	fmt.Println("ToAdmin")
}

//Notification of app message being sent to target.
func (a *Application) ToApp(message *quickfix.Message, sessionID quickfix.SessionID) error {
	fmt.Println("ToApp")
	return nil
}

//Notification of admin message being received from target.
func (a *Application) FromAdmin(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Println("FromAdmin")
	return nil
}

//Notification of app message being received from target.
func (a *Application) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	return a.Route(message, sessionID)
}

type executor struct {
	*quickfix.MessageRouter
}

var cfgFileName = flag.String("cfg", "config.cfg", "Acceptor config file")

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

	app := newApp()
	global := appSettings.SessionSettings()
	logFactory := quickfix.NewScreenLogFactory()
	for k, v := range global {
		if k.BeginString == quickfix.BeginStringFIX42 {
			app.setting = v
		}
	}

	quickApp, err := quickfix.NewInitiator(app, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
	if err != nil {
		return fmt.Errorf("Unable to create Acceptor: %s\n", err)
	}

	err = quickApp.Start()
	if err != nil {
		return fmt.Errorf("Unable to start Acceptor: %s\n", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	quickApp.Stop()

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

	clientID, err := app.setting.Setting("ClientID")
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

	request.Header.SetString(quickfix.Tag(56), target)
	request.Header.SetString(quickfix.Tag(49), sender)
	request.Header.SetString(quickfix.Tag(109), clientID)

	return request.ToMessage()
}

func (app *Application) makeFix42Logon() *quickfix.Message {
	password, err := app.setting.Setting("Password")
	if err != nil {
		panic(fmt.Sprintf("Miss SenderCompID %+v", err))
	}

	request := fix42lo.New(field.NewEncryptMethod(enum.EncryptMethod_NONE_OTHER), field.NewHeartBtInt(5))
	request.Header.SetString(quickfix.Tag(554), password)

	return request.ToMessage()
}
