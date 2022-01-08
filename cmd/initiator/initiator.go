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
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/quickfix"
)

type Application struct{}

//Notification of a session begin created.
func (a *Application) OnCreate(sessionID quickfix.SessionID) {
	fmt.Println(">>> OnCreate")
	logon := makeNewLogon()
	err := quickfix.SendToTarget(logon, sessionID)

	if err != nil {
		fmt.Printf("Error Sending logon: %s,", err)
	} else {
		fmt.Println("Logon ok: ", sessionID)
	}

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
	fmt.Printf("FromApp: %s\n", message.String())
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
	app := Application{}

	printConfig(bytes.NewReader(stringData))

	initiator, err := quickfix.NewInitiator(&app, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
	if err != nil {
		return fmt.Errorf("Error NewInitiator : %s,", err)
	}

	err = initiator.Start()
	if err != nil {
		return fmt.Errorf("Error Start : %s,", err)
	}
	// logon := makeNewLogon()
	// err = quickfix.Send(logon)

	// if err != nil {
	// 	return fmt.Errorf("Error Sending logon: %s,", err)
	// }

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

func makeNewOdMsg() *quickfix.Message {
	rawB := []byte(`8=FIX.4.49=14835=D34=108049=TESTBUY152=20180920-18:14:19.50856=TESTSELL111=63673064027889863415=USD21=238=700040=154=155=MSFT60=20180920-18:14:19.49210=092`)
	buf := bytes.NewBuffer(rawB)
	msg := quickfix.NewMessage()
	err := quickfix.ParseMessage(msg, buf)
	if err != nil {
		panic(err)
	}
	setHeader(&msg.Header)

	return msg.ToMessage()
}

func makeNewLogon() *quickfix.Message {
	rawB := []byte(
		`8=FIX.4.49=7535=A34=109249=TESTBUY152=20210920-18:24:59.64356=TESTSELL198=0108=6010=178`,
	)
	buf := bytes.NewBuffer(rawB)
	msg := quickfix.NewMessage()
	err := quickfix.ParseMessage(msg, buf)
	if err != nil {
		panic(err)
	}
	setHeader(&msg.Header)

	return msg.ToMessage()
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
