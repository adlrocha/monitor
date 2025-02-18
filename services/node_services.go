package services

import (
	"crypto/tls"
	"errors"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alastria/monitor/lib"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/httplib"
	"github.com/astaxie/beego/logs"
)

// URI to status message
const status = "/v1/monitor/status"
const versionupdate = "/v1/monitor/update"

// URI to logjson message
const logsjson = "/v1/node/info/json"

const proposeURI = "/v1/node/propose"
const coinbaseURI = "/v1/node/coinbase"

const update = "/v1/node/update"

var apppath string
var err error
var log *logs.BeeLogger

var validators = regexp.MustCompile(`\| +(.*) +\| +(.*) +\| +.* +\| +enode://(.*)@(.*):(.*)\?discport.* +\|`)
var regulars = regexp.MustCompile(`\| *(.*) *\| *(.*) *\| *.* *\| *(.*=) *\| *enode://([a-z0-9]*)@([0-9\.]*):([0-9]*)\?[a-z=0]+ *\|`)
var emails = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
var nodeInfo_reg = regexp.MustCompile(`nodeInfo.*id\: \\"([a-z0-9]*)\\".*name: \\"([A-Za-z\/\.\-0-9]*)\\"`)
var peer_reg = regexp.MustCompile(`{\\n *caps: \[\\"istanbul\/64\\"],\\n +id: \\"([a-z0-9]*)\\",\\n +name: \\"([A-Za-z\/\.\-0-9]*)\\",\\n +network: {\\n +localAddress: \\"[0-9\.:]*\\",\\n +remoteAddress: \\"([0-9\.]*):[0-9]+\\"\\n +},\\n +protocols: {\\n +istanbul: {\\n +difficulty: [0-9]*,\\n +head: \\"[0-9a-z]*\\",\\n +version: [0-9]+\\n +}\\n +}\\n}`)
var coinbase = regexp.MustCompile(`.*\"coinbase\": \"\\\"(.*)\\\"\\n\",`)
var getvalidators = regexp.MustCompile(`.*\"getValidators\": \"\[(.*)\]`)
var eachvalidator = regexp.MustCompile(`\\\"([0-9a-zA-Z]+)\\\",*`)

type Nodo struct {
	Entidad     string    `json: "entidad"`
	Contactos   []string  `json: "contactos"`
	Enode       string    `json: "enode"`
	IP          string    `json: "ip"`
	Port        string    `json: "port"`
	PrivateFor  string    `json: "privateFor"`
	Coinbase    string    `json: "coinbase"`
	Monitor     string    `json: "monitor"`
	Peers       []Peer    `json: "peers"`
	Validators  []string  `json: "validators"`
	Incidencias string    `json: "incidencias"`
	LastUpdate  time.Time `json: "lastUpdate"`
}

type Monitor struct {
	BlockNumber       string `json: "blockNumber"`
	Candidates        string `json: "candidates"`
	Validators        string `json: "getValidators"`
	Mining            string `json: "mining"`
	NetworkID         string `json: "netVersion"`
	NodeInfo          string `json: "nodeInfo"`
	PeerCount         string `json: "peerCount"`
	Peers             []Peer `json: "peers"`
	Pending           string `json: "pendingTransactions`
	Port              string `json: "port1"`
	RpcPort           string `json: "port2"`
	ConstellationPort string `json: "port3"`
	Syncing           bool   `json: "syncing"`
	TxPool            string `json: "txPool"`
}

type Peer struct {
	Caps      []string     `json: "caps"`
	Id        string       `json: "id"`
	Name      string       `json: "name"`
	Network   PeerNetwork  `json: "network"`
	Protocols PeerProtocol `json: "protocols"`
}

type PeerNetwork struct {
	LocalAddress  string `json: "localAddress"`
	RemoteAddress string `json: "remoteAddress"`
}

type PeerProtocol struct {
	Istanbul PeerIstanbul `json: "istanbul"`
}

type PeerIstanbul struct {
	Difficulty int    `json: "difficulty"`
	Head       string `json: "head"`
	Version    int    `json: "version"`
}

type StatusReturn struct {
	Status string `json: "status"`
}

type CoinbaseReturn struct {
	Data string `json: "data"`
}
type NodeServices struct {
	nodos      []Nodo
	validators []Nodo
	cert       tls.Certificate
	visited    map[string]bool
	set        map[string]*Nodo
	all        map[string]*Nodo
}

func init() {
	_, file, _, _ := runtime.Caller(1)
	apppath, _ = filepath.Abs(filepath.Dir(filepath.Join(file, ".."+string(filepath.Separator))))
	log = logs.GetBeeLogger()
}

// By default `ibft``
func NewNodeServices(feature string) (node *NodeServices) {
	node = new(NodeServices)
	nd := node.GetValidatorDirectory("https://raw.githubusercontent.com/alastria/alastria-node/feature/" + feature + "/DIRECTORY_VALIDATOR.md")
	for cont := 0; cont < len(nd); cont++ {
		node.validators = append(node.nodos, nd[cont])
		node.nodos = append(node.nodos, nd[cont])
	}
	nd = node.GetGeneralDirectory("https://raw.githubusercontent.com/alastria/alastria-node/feature/" + feature + "/DIRECTORY_REGULAR.md")
	for cont := 0; cont < len(nd); cont++ {
		node.nodos = append(node.nodos, nd[cont])
	}

	// http://www.levigross.com/2015/11/21/mutual-tls-authentication-in-go/
	// Load our TLS key pair to use for authentication
	node.cert, err = tls.LoadX509KeyPair(
		apppath+beego.AppConfig.String("TLSClientCertFile"),
		apppath+beego.AppConfig.String("TLSClientKeyFile"),
	)

	if err != nil {
		log.Error("Unable to load cert", err)
	}
	return node
}

// uri example: /v1/monitor/status
func (n *NodeServices) call(ip string, uri string) (response string, err error) {
	// https://beego.me/docs/module/httplib.md

	//log.SetFlags(log.Lshortfile)
	//beego.Trace("Get config")
	req := httplib.Get("https://" + ip + ":8443" + uri)
	//beego.Trace("TLS Config")
	req.SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{n.cert},
	})
	//beego.Trace("Running")
	response, err = req.Debug(true).String()

	//beego.Info("response: %s", response, "error: %s", err)

	return
}

// uri example: /v1/monitor/status
func (n *NodeServices) getJSON(response interface{}, ip string, uri string) (err error) {
	// https://beego.me/docs/module/httplib.md

	//log.SetFlags(log.Lshortfile)
	//beego.Trace("Get config")
	req := httplib.Get("https://" + ip + ":8443" + uri)
	//beego.Trace("TLS Config")
	req.SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{n.cert},
	})
	//beego.Trace("Running")
	err = req.Debug(true).ToJSON(&response)

	//beego.Info("response: %s", response, "error: %s", err)

	return
}

func (n *NodeServices) GetValidatorDirectory(url string) (nodos []Nodo) {
	// "https://raw.githubusercontent.com/alastria/alastria-node/feature/ibft/DIRECTORY_VALIDATOR.md"
	stfile, _ := lib.GetGithub(url)
	result := validators.FindAllStringSubmatch(lib.GetFile(stfile), -1)
	nodos = make([]Nodo, len(result))
	for cont := 0; cont < len(result); cont++ {
		nodos[cont].Entidad = strings.TrimSpace(result[cont][1])
		mails := emails.FindAllStringSubmatch(result[cont][2], -1)
		nodos[cont].Contactos = make([]string, len(mails))
		for cmail := 0; cmail < len(mails); cmail++ {
			nodos[cont].Contactos[cmail] = strings.TrimSpace(mails[cmail][0])
		}
		nodos[cont].Enode = strings.TrimSpace(result[cont][3])
		nodos[cont].IP = strings.TrimSpace(result[cont][4])
		nodos[cont].Port = strings.TrimSpace(result[cont][5])
	}
	return
}

func (n *NodeServices) GetGeneralDirectory(url string) (nodos []Nodo) {
	// "https://raw.githubusercontent.com/alastria/alastria-node/feature/ibft/DIRECTORY_REGULAR.md"
	stfile, _ := lib.GetGithub(url)
	result := regulars.FindAllStringSubmatch(lib.GetFile(stfile), -1)
	nodos = make([]Nodo, len(result))
	for cont := 0; cont < len(result); cont++ {
		nodos[cont].Entidad = strings.TrimSpace(result[cont][1])
		mails := emails.FindAllStringSubmatch(result[cont][2], -1)
		nodos[cont].Contactos = make([]string, len(mails))
		for cmail := 0; cmail < len(mails); cmail++ {
			nodos[cont].Contactos[cmail] = strings.TrimSpace(mails[cmail][0])
		}
		nodos[cont].PrivateFor = strings.TrimSpace(result[cont][3])
		nodos[cont].Enode = strings.TrimSpace(result[cont][4])
		nodos[cont].IP = strings.TrimSpace(result[cont][5])
		nodos[cont].Port = strings.TrimSpace(result[cont][6])
	}
	return
}

func (n *NodeServices) CheckPermission() (problems []Nodo) {
	n.visited, n.all = n.getSets()
	nodo, err := n.GetFirstValidatorUp()
	n.set = make(map[string]*Nodo)
	if err == nil {
		n.set[nodo.Enode] = &nodo
		for len(n.set) > 0 {
			for enode := range n.set {
				log.Trace("Comenzando a verificar el nodo: %s.", n.set[enode].Entidad)
				aux := n.set[enode]
				n.nodeVerify(aux)
				if len(aux.Incidencias) > 0 {
					log.Trace("Detectada una incidencia en el nodo.")
				}
			}
		}
	}

	log.Trace("Recopilando las incidencias.")
	fecha := time.Now()
	for key := range n.all {
		aux := n.all[key]
		aux.LastUpdate = fecha
		if !n.visited[key] {
			log.Trace("Parece que el nodo está fuera de línea. %s", aux)
			aux.Incidencias += "\n [*] Parece que el nodo está fuera de línea."
		}
		if len(aux.Incidencias) > 0 {
			problems = append(problems, *aux)
		}
	}
	return
}

func (n *NodeServices) nodeVerify(nodo *Nodo) {
	n.visited[nodo.Enode] = true
	delete(n.set, nodo.Enode)
	if n.IsUpAndRunning(*nodo) {
		json, err := n.call(nodo.IP, logsjson)
		if err == nil {
			nodo.Monitor = json
			n.nodeInfoVerify(nodo)
			n.peersVerify(nodo)
			n.ibftExtraction(nodo)
			n.all[nodo.Enode] = nodo
		}
	} else {
		log.Trace("Parece que el monitor no está disponible. %s", nodo)
		nodo.Incidencias += "\n [*] Parece que el monitor no está disponible."
	}
}

func (n *NodeServices) nodeInfoVerify(nodo *Nodo) {
	nodeInfos := nodeInfo_reg.FindAllStringSubmatch(nodo.Monitor, -1)
	if nodeInfos[0][1] != nodo.Enode {
		log.Trace("El enode no coincide %s", nodo)
		nodo.Incidencias += "[*] El Enode no coincide."
	}
	/*if !strings.Contains(nodeInfos[0][2], nodo.Entidad) {
		log.Trace("El nombre asignado al nodo en el directorio no está incluido en el identificador del nodo. %s", nodo)
		nodo.Incidencias += "\n [*] El nombre asignado al nodo en el directorio no está incluido en el identificador del nodo."
	}*/
}

func (n *NodeServices) ibftExtraction(nodo *Nodo) {
	if len(nodo.PrivateFor) == 0 && len(nodo.Coinbase) == 0 {
		coinbases := coinbase.FindAllStringSubmatch(nodo.Monitor, -1)
		if coinbases[0][1] != nodo.Coinbase {
			nodo.Coinbase = coinbases[0][1]
		}
		ibftgetvalidators := getvalidators.FindAllStringSubmatch(nodo.Monitor, -1)
		if len(ibftgetvalidators[0][1]) > 0 {
			ibftvalidators := eachvalidator.FindAllStringSubmatch(ibftgetvalidators[0][1], -1)
			for cont := 0; cont < len(ibftvalidators); cont++ {
				nodo.Validators = append(nodo.Validators, ibftvalidators[cont][1])
			}
			sort.Strings(nodo.Validators)
		}
	}
}

func (n *NodeServices) peersVerify(nodo *Nodo) {
	peers := peer_reg.FindAllStringSubmatch(nodo.Monitor, -1)
	for cont := 0; cont < len(peers); cont++ {
		peer := peers[cont]
		var aux Peer
		aux.Id = peer[1]
		aux.Name = peer[2]
		aux.Network = PeerNetwork{}
		aux.Network.RemoteAddress = peer[3]
		nodo.Peers = append(nodo.Peers, aux)
		if aux, ok := n.all[peer[1]]; ok {
			if !n.visited[aux.Enode] {
				n.set[aux.Enode] = aux
			}
		} else {
			log.Trace("El enode "+peer[1]+", no es conocido. %s - %s.", nodo.Entidad, nodo.Enode)
			nodo.Incidencias += "\n [*] El enode " + peer[1] + ", no es conocido."
		}
	}
}

func (n *NodeServices) IsUpAndRunning(nodo Nodo) (ok bool) {
	var retorno StatusReturn
	ok = false
	err := n.getJSON(&retorno, nodo.IP, status)
	if err == nil {
		ok = retorno.Status == "ok"
	}
	return
}

type proposeForm struct {
	Candidate string
	Value     string
}

func (n *NodeServices) ProposeSingleNode(nodo *Nodo, address string, value bool) (ok bool) {

	log.Info("Entidad: %s -  %s: propose(%s, %s)", nodo.Entidad, nodo.Coinbase, address, value)

	req := httplib.Post("https://" + nodo.IP + ":8443" + proposeURI)

	req.SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{n.cert},
	})
	req.JSONBody(proposeForm{address, strconv.FormatBool(value)})
	var retorno StatusReturn
	ok = false

	err = req.Debug(true).ToJSON(&retorno)

	if err == nil {
		ok = retorno.Status == "ok"
	}
	return
}

func (n *NodeServices) ListVolunteers() (validators []*Nodo) {
	for key := range n.all {
		aux := n.all[key]

		if n.visited[key] && len(aux.Coinbase) > 0 {
			validators = append(validators, aux)
		}
	}
	return
}

func (n *NodeServices) ListValidators() (validators []*Nodo) {
	for key := range n.all {
		aux := n.all[key]

		if n.visited[key] && len(aux.Coinbase) > 0 {

			i := sort.SearchStrings(aux.Validators, aux.Coinbase)

			if i < len(aux.Validators) && aux.Validators[i] == aux.Coinbase {
				validators = append(validators, aux)
				log.Info("Validator: %s: %s - %s", aux.Entidad, aux.Coinbase, aux.Validators)
			}
		}
	}
	return
}

func (n *NodeServices) ProposeNodes(iskey string) (ok bool) {
	validators := n.ListValidators()
	for key := range validators {
		aux := validators[key]

		i := sort.SearchStrings(aux.Validators, iskey)
		proposal := !(i < len(aux.Validators) && aux.Validators[i] == iskey)

		if strings.Compare(iskey, aux.Coinbase) != 0 {
			ok = n.ProposeSingleNode(aux, iskey, proposal) || ok
		}
	}

	return
}

func (n *NodeServices) VersionUpdate() (ok bool) {
	for key := range n.all {
		aux := n.all[key]

		if n.visited[key] {
			n.call(aux.IP, versionupdate)
		}
	}
	return
}

func (n *NodeServices) Update() (ok bool) {
	for key := range n.all {
		aux := n.all[key]

		if n.visited[key] {
			n.call(aux.IP, update)
		}
	}
	return
}

func (n *NodeServices) GetCoinbase(nodo Nodo) (coinbase string) {

	req := httplib.Get("https://" + nodo.IP + ":8443" + coinbaseURI)
	log.Trace("Requesting coinbase at: %s", ("https://" + nodo.IP + ":8443" + coinbaseURI))

	req.SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{n.cert},
	})
	var retorno CoinbaseReturn
	coinbase = "false"

	err = req.Debug(true).ToJSON(&retorno)

	if err == nil {
		coinbase = retorno.Data
		log.Trace("Coinbase obtained in GET %s", coinbase)
	}
	return
}

func (n *NodeServices) GetFirstValidatorUp() (nodo Nodo, err error) {
	var ok bool = false
	var cont int = 0

	for !ok && cont < len(n.nodos) {
		nodo = n.nodos[cont]
		// is validator
		if nodo.PrivateFor == "" && n.IsUpAndRunning(nodo) {
			ok = true
		} else {
			cont++
		}
	}
	if !ok {
		err = errors.New("No se ha encontrado ningún monitor en ningún nodo validador.")
	}
	return
}

func (n *NodeServices) getSets() (visited map[string]bool, set map[string]*Nodo) {
	visited = make(map[string]bool)
	set = make(map[string]*Nodo)
	for cont := 0; cont < len(n.nodos); cont++ {
		var tmp = n.nodos[cont]
		set[tmp.Enode] = &tmp
		visited[tmp.Enode] = false
	}
	return
}

func (n *NodeServices) Calls(ips []string, uris []string) (err error) {
	var uri string
	var ip string
	for contUri := 0; contUri < len(uris); contUri++ {
		uri = uris[contUri]
		for contIp := 0; contIp < len(ips); contIp++ {
			ip = ips[contIp]
			_, err = n.call(ip, uri)
			if err != nil {
				contUri = len(uris)
				contIp = len(ips)
			}
		}
	}
	return
}
