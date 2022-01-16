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

	"github.com/davecgh/go-spew/spew"
	fix42er "github.com/quickfixgo/fix42/executionreport"
	fix42lo "github.com/quickfixgo/fix42/logon"
	fix42mdir "github.com/quickfixgo/fix42/marketdataincrementalrefresh"
	fix42mdr "github.com/quickfixgo/fix42/marketdatarequest"
	fix42mdrr "github.com/quickfixgo/fix42/marketdatarequestreject"
	fix42mdsfr "github.com/quickfixgo/fix42/marketdatasnapshotfullrefresh"
	fix42sd "github.com/quickfixgo/fix42/securitydefinition"
	fix42sdr "github.com/quickfixgo/fix42/securitydefinitionrequest"
	"github.com/quickfixgo/quickfix"
)

type Application struct {
	mdReqID    int
	securityID int
	sessionID  chan quickfix.SessionID
	symbols    map[string]string
	mu         sync.Mutex
	setting    *quickfix.SessionSettings
	*quickfix.MessageRouter
}

func (app *Application) genSecurityID() field.SecurityReqIDField {
	app.securityID++
	return field.NewSecurityReqID(strconv.Itoa(app.securityID))
}

func (app *Application) genMDID() field.MDReqIDField {
	app.mdReqID++
	return field.NewMDReqID(strconv.Itoa(app.mdReqID))
}

func newApp() *Application {
	app := Application{
		MessageRouter: quickfix.NewMessageRouter(),
		symbols:       make(map[string]string),
		sessionID:     make(chan quickfix.SessionID, 1),
	}
	app.AddRoute(fix42er.Route(app.OnFIX42ExecutionReport))
	app.AddRoute(fix42sd.Route(app.OnFIX42SecurityDefinition))
	app.AddRoute(fix42mdir.Route(app.OnFIX42MarketDataIncrementalRefresh))
	app.AddRoute(fix42mdrr.Route(app.OnFIX42MarketDataRequestReject))
	app.AddRoute(fix42mdsfr.Route(app.OnFIX42MarketDataSnapshotFullRefresh))
	app.AddRoute(fix42mdr.Route(app.OnFIX42MarketDataRequest))

	return &app
}

func (a *Application) OnFIX42MarketDataIncrementalRefresh(msg fix42mdir.MarketDataIncrementalRefresh, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf("ON OnFIX42MarketDataIncrementalRefresh \n %+v \n", msg)
	if price, err := msg.GetString(quickfix.Tag(270)); err != nil {
		fmt.Println("GOT YOU >>>>>>>>>>>>>>>>>>>>> ", price)
	}
	panic("PANICE TO SEE ME")
	if entries, err := msg.GetNoMDEntries(); err == nil {
		spew.Dump(entries)
	}

	return nil
}

func (a *Application) OnFIX42MarketDataRequest(msg fix42mdr.MarketDataRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf("ON MarketDataRequest%+v \n", msg)
	return nil
}

func (a *Application) OnFIX42MarketDataRequestReject(msg fix42mdrr.MarketDataRequestReject, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf("ON OnFIX42MarketDataRequestReject %+v \n", msg)
	return nil
}

func (a *Application) OnFIX42MarketDataSnapshotFullRefresh(msg fix42mdsfr.MarketDataSnapshotFullRefresh, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf("ON OnFIX42MarketDataSnapshotFullRefresh \n %+v \n", msg)

	if price, err := msg.GetString(quickfix.Tag(270)); err != nil {
		fmt.Println("GOT YOU >>>>>>>>>>>>>>>>>>>>> ", price)
	}
	return nil
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
	msg := fix42sdr.New(a.genSecurityID(), field.NewSecurityRequestType(enum.SecurityRequestType_SYMBOL))
	a.sessionID <- sessionID
	err := quickfix.SendToTarget(msg, sessionID)
	if err != nil {
		fmt.Printf("Error SendToTarget : %s,", err)
	} else {
		fmt.Printf("\nSend ok %+v \n", msg)
	}
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
}

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
	fmt.Printf("\n>>>>>>>> [%+v] \n", message)
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

	go app.crank()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	quickApp.Stop()

	return nil
}

func (app *Application) crank() {
	sessionID := <-app.sessionID

	tick := time.Tick(30 * time.Second)
	for {
		<-tick
		// NOTED: Tick only SOL now for testing
		if symbol, ok := app.symbols["BCHUSD"]; ok {
			msg := app.makeFix42MarketDataRequest(symbol)
			err := quickfix.SendToTarget(msg, sessionID)
			fmt.Printf("Send makeFix42MarketDataRequest %+v ", msg.String())
			if err != nil {
				fmt.Printf("XXX> Error SendToTarget : %s,", err)
			} else {
				fmt.Printf("===> Send ok %+v \n", msg)
			}
		}
	}
}

func (app *Application) makeFix42MarketDataIncrementalRefresh(symbol string) *quickfix.Message {
	_msg := app.makeFix42MarketDataRequest("SOLUSD")

	request := fix42mdir.FromMessage(_msg)

	return request.ToMessage()
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

	mdID := app.genMDID()
	request := fix42mdr.New(mdID,
		field.NewSubscriptionRequestType(enum.SubscriptionRequestType_SNAPSHOT_PLUS_UPDATES),
		field.NewMarketDepth(0),
	)
	request.SetSenderCompID(sender)
	request.SetTargetCompID(target)
	request.Header.SetString(quickfix.Tag(109), clientID)

	// entryTypes := fix42mdr.NewNoMDEntryTypesRepeatingGroup()
	// entryTypes.Add().SetMDEntryType(enum.MDEntryType_FIXING_PRICE)
	// request.SetNoMDEntryTypes(entryTypes)

	relatedSym := fix42mdr.NewNoRelatedSymRepeatingGroup()
	relatedSym.Add().SetSymbol(symbol)
	request.SetNoRelatedSym(relatedSym)

	// request.SetString(quickfix.Tag(146), "1")
	request.SetString(quickfix.Tag(5000), "0")

	// request.SetMDUpdateType(enum.MDUpdateType_FULL_REFRESH)

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
