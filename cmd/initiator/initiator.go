package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	fix44er "github.com/quickfixgo/fix44/executionreport"
	fix44mdr "github.com/quickfixgo/fix44/marketdatarequest"
	"github.com/quickfixgo/quickfix"
	"github.com/shopspring/decimal"
)

func makeMarketDataRequest44() fix44mdr.MarketDataRequest {
	request := fix44mdr.New(field.NewMDReqID("MARKETDATAID"),
		field.NewSubscriptionRequestType(enum.SubscriptionRequestType_SNAPSHOT),
		field.NewMarketDepth(0),
	)

	entryTypes := fix44mdr.NewNoMDEntryTypesRepeatingGroup()
	entryTypes.Add().SetMDEntryType(enum.MDEntryType_BID)
	request.SetNoMDEntryTypes(entryTypes)

	relatedSym := fix44mdr.NewNoRelatedSymRepeatingGroup()
	relatedSym.Add().SetSymbol("LNUX")
	request.SetNoRelatedSym(relatedSym)

	setHeader(request.Header)
	return request
}

type Application struct {
	orderID int
	execID  int
	*quickfix.MessageRouter
}

func newApp() *Application {
	app := Application{
		MessageRouter: quickfix.NewMessageRouter(),
	}
	app.AddRoute(fix44er.Route(app.OnFIX44ExecutionReport))

	return &app
}

func (a *Application) OnFIX44ExecutionReport(msg fix44er.ExecutionReport, sessionID quickfix.SessionID) quickfix.MessageRejectError {

	fmt.Printf(">>> OnFIX44ExecutionReport %+v", msg)
	return nil
}

//Notification of a session begin created.
func (a *Application) OnCreate(sessionID quickfix.SessionID) {

}

//Notification of a session successfully logging on.
func (a *Application) OnLogon(sessionID quickfix.SessionID) {
	fmt.Println("Onlongon")

}

//Notification of a session logging off or disconnecting.
func (a *Application) OnLogout(sessionID quickfix.SessionID) {}

//Notification of admin message being sent to target.
func (a *Application) ToAdmin(message *quickfix.Message, sessionID quickfix.SessionID) {}

//Notification of app message being sent to target.
func (a *Application) ToApp(message *quickfix.Message, sessionID quickfix.SessionID) error {
	fmt.Printf("Sending %s\n", message)
	return nil
}

//Notification of admin message being received from target.
func (a *Application) FromAdmin(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	return nil
}

//Notification of app message being received from target.
func (a *Application) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	fmt.Printf(">>>>>>>>>> I GOT YOU \n")
	fmt.Printf(">>>>>>>> FromApp: %s\n", message.String())
	fmt.Printf("=============================== \n")
	return nil
}

var cfgFileName = flag.String("cfg", "initiator.cfg", "Acceptor config file")

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

	printConfig(bytes.NewReader(stringData))

	initiator, err := quickfix.NewInitiator(app, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
	if err != nil {
		return fmt.Errorf("Error NewInitiator : %s,", err)
	}

	err = initiator.Start()
	if err != nil {
		return fmt.Errorf("Error Start : %s,", err)
	}

	global := appSettings.SessionSettings()
	for k := range global {
		if k.BeginString == quickfix.BeginStringFIX44 {
			time.Sleep(5 * time.Second)
			msg := makeMarketDataRequest44()
			err := quickfix.SendToTarget(msg, k)

			if err != nil {
				return fmt.Errorf("Error SendToTarget : %s,", err)
			}
		}
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt

	initiator.Stop()

	return nil
}

func queryString(fieldName string) string {
	fmt.Printf("%v: ", fieldName)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return scanner.Text()
}

type header interface {
	Set(f quickfix.FieldWriter) *quickfix.FieldMap
}

func setHeader(h header) {
	h.Set(senderCompID("TESTBUY1"))
	h.Set(targetCompID("TESTSELL1"))
	// if ok := queryConfirm("Use a TargetSubID"); !ok {
	// 	return
	// }

	// h.Set(queryTargetSubID())
}

func queryTargetSubID() field.TargetSubIDField {
	return field.NewTargetSubID(queryString("TargetSubID"))
}

func queryConfirm(prompt string) bool {
	fmt.Println()
	fmt.Printf("%v?: ", prompt)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	return strings.ToUpper(scanner.Text()) == "Y"
}

func targetCompID(v string) field.TargetCompIDField {
	return field.NewTargetCompID(v)
}

func senderCompID(v string) field.SenderCompIDField {
	return field.NewSenderCompID(v)
}

func printConfig(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	color.Set(color.Bold)
	fmt.Println("Started FIX Acceptor with config:")
	color.Unset()

	color.Set(color.FgHiMagenta)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}

	color.Unset()
}
func (e *Application) genOrderID() field.OrderIDField {
	e.orderID++
	return field.NewOrderID(strconv.Itoa(e.orderID))
}

func (e *Application) genExecID() field.ExecIDField {
	e.execID++
	return field.NewExecID(strconv.Itoa(e.execID))
}
func (e *Application) makeExecutorReport(msg fix44mdr.MarketDataRequest) *quickfix.Message {
	execReport := fix44er.New(
		e.genOrderID(),
		e.genExecID(),
		field.NewExecType(enum.ExecType_FILL),
		field.NewOrdStatus(enum.OrdStatus_FILLED),
		field.NewSide(enum.Side_BUY),
		field.NewLeavesQty(decimal.Zero, 2),
		field.NewCumQty(decimal.Decimal{}, 2),
		field.NewAvgPx(decimal.Decimal{}, 2),
	)

	_msg := execReport.ToMessage()
	setHeader(&_msg.Header)

	return _msg
}
