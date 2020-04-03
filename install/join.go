package install

import (
	"fmt"
	"github.com/fanux/sealos/ipvs"
	"github.com/wonderivan/logger"
	"os"
	"strings"
	"sync"
)

//BuildJoin is
func BuildJoin(joinMasters, joinNodes []string) {
	if len(joinMasters) > 0 {
		joinMastersFunc(joinMasters)
	}
	if len(joinNodes) > 0 {
		joinNodesFunc(joinNodes)
	}
}

func joinMastersFunc(joinMasters []string) {
	masters := MasterIPs
	nodes := NodeIPs
	i := &SealosInstaller{
		Hosts:   joinMasters,
		Masters: masters,
		Nodes:   nodes,
	}
	i.CheckValid()
	i.SendPackage()
	i.GeneratorCerts()
	i.JoinMasters(joinMasters)
	//master join to MasterIPs
	MasterIPs = append(MasterIPs, joinMasters...)
	i.lvscare()

}

//joinNodesFunc is join nodes func
func joinNodesFunc(joinNodes []string) {
	// 所有node节点
	nodes := joinNodes
	i := &SealosInstaller{
		Hosts:   nodes,
		Masters: MasterIPs,
		Nodes:   nodes,
	}
	i.CheckValid()
	i.SendPackage()
	i.GeneratorToken()
	i.JoinNodes()
	//node join to NodeIPs
	NodeIPs = append(NodeIPs, joinNodes...)
}

//GeneratorToken is
//这里主要是为了获取CertificateKey
func (s *SealosInstaller) GeneratorCerts() {
	cmd := `kubeadm init phase upload-certs --upload-certs`
	output := SSHConfig.CmdToString(s.Masters[0], cmd, "\r\n")
	logger.Debug("[globals]decodeCertCmd: %s", output)
	slice := strings.Split(output, "Using certificate key:\r\n")
	slice1 := strings.Split(slice[1], "\r\n")
	CertificateKey = slice1[0]
	cmd = "kubeadm token create --print-join-command"
	out := SSHConfig.Cmd(s.Masters[0], cmd)
	decodeOutput(out)
}

//GeneratorToken is
func (s *SealosInstaller) GeneratorToken() {
	cmd := `kubeadm token create --print-join-command`
	output := SSHConfig.Cmd(s.Masters[0], cmd)
	decodeOutput(output)
}

//JoinMasters is
func (s *SealosInstaller) JoinMasters(masters []string) {
	//copy certs
	//for _, master := range masters {
	//
	//	SendPackage(sealos, s.Hosts, "/usr/bin", &beforeHook, &afterHook)
	//}
	//SendPackage

	//join master do sth
	cmd := s.Command(Version, JoinMaster)
	for _, master := range masters {
		cmdHosts := fmt.Sprintf("echo %s %s >> /etc/hosts", IpFormat(s.Masters[0]), ApiServer)
		_ = SSHConfig.CmdAsync(master, cmdHosts)
		_ = SSHConfig.CmdAsync(master, cmd)
		cmdHosts = fmt.Sprintf(`sed "s/%s/%s/g" -i /etc/hosts`, IpFormat(s.Masters[0]), IpFormat(master))
		_ = SSHConfig.CmdAsync(master, cmdHosts)
		copyk8sConf := `mkdir -p /root/.kube && cp -i /etc/kubernetes/admin.conf /root/.kube/config`
		_ = SSHConfig.CmdAsync(master, copyk8sConf)
		cleaninstall := `rm -rf /root/kube`
		_ = SSHConfig.CmdAsync(master, cleaninstall)
	}
}

//JoinNodes is
func (s *SealosInstaller) JoinNodes() {
	var masters string
	var wg sync.WaitGroup
	for _, master := range s.Masters {
		masters += fmt.Sprintf(" --rs %s:6443", IpFormat(master))
	}
	ipvsCmd := fmt.Sprintf("sealos ipvs --vs %s:6443 %s --health-path /healthz --health-schem https --run-once", VIP, masters)

	for _, node := range s.Nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			cmdHosts := fmt.Sprintf("echo %s %s >> /etc/hosts", VIP, ApiServer)
			_ = SSHConfig.CmdAsync(node, cmdHosts)
			_ = SSHConfig.CmdAsync(node, ipvsCmd) // create ipvs rules before we join node
			cmd := s.Command(Version, JoinNode)
			//create lvscare static pod
			yaml := ipvs.LvsStaticPodYaml(VIP, MasterIPs, "")
			_ = SSHConfig.CmdAsync(node, cmd)
			_ = SSHConfig.CmdAsync(node, fmt.Sprintf("mkdir -p /etc/kubernetes/manifests && echo '%s' > /etc/kubernetes/manifests/kube-sealyun-lvscare.yaml", yaml))

			cleaninstall := `rm -rf /root/kube`
			_ = SSHConfig.CmdAsync(node, cleaninstall)
		}(node)
	}

	wg.Wait()
}

func (s *SealosInstaller) lvscare() {
	var wg sync.WaitGroup
	for _, node := range s.Nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			yaml := ipvs.LvsStaticPodYaml(VIP, MasterIPs, "")
			_ = SSHConfig.CmdAsync(node, "rm -rf  /etc/kubernetes/manifests/kube-sealyun-lvscare*")
			_ = SSHConfig.CmdAsync(node, fmt.Sprintf("mkdir -p /etc/kubernetes/manifests && echo '%s' > /etc/kubernetes/manifests/kube-sealyun-lvscare.yaml", yaml))
		}(node)
	}

	wg.Wait()
}

func (s *SealosInstaller) sendCerts(hosts []string) {
	//cert generator in sealos
	//get abs path
	home, _ := os.UserHomeDir()
	certPath := home + defaultConfigPath + defaultCertPath
	certEtcdPath := home + defaultConfigPath + defaultCertEtcdPath
	print(certEtcdPath, certPath)
	//
	//caConfigs := cert.CaList(certPath, certEtcdPath)
	//SendPackage(certPath + "/sa.key",hosts,certPath,nil,nil)
	//SendPackage(certPath + "/sa.pub",hosts,certPath,nil,nil)
	//for _, ca := range caConfigs {
	//	SendPackage(ca.Path + "/" +ca.BaseName +".key",hosts,ca.Path,nil,nil)
	//	SendPackage(ca.Path + "/" +ca.BaseName +".crt",hosts,ca.Path,nil,nil)
	//}
}
func (s *SealosInstaller) sendCaCerts(hosts []string) {
	//get abs path
	home, _ := os.UserHomeDir()
	certPath := home + defaultConfigPath + defaultCertPath
	certEtcdPath := home + defaultConfigPath + defaultCertEtcdPath
	print(certEtcdPath, certPath)
	//
	//certConfigs:=cert.CertList(certPath, certEtcdPath)
	//for _, cert := range certConfigs {
	//	SendPackage(cert.Path + "/" +cert.BaseName +".key",hosts,cert.Path,nil,nil)
	//	SendPackage(cert.Path + "/" +cert.BaseName +".crt",hosts,cert.Path,nil,nil)
	//}
}
