package host

import (
	gdxthreads "github.com/DxChainNetwork/godx/common/threadManager"
	"github.com/DxChainNetwork/godx/host/storageManager"
)

type Host struct{
	//// RPC Metrics - atomic variables need to be placed at the top to preserve
	//// compatibility with 32bit systems. These values are not persistent.
	//atomicDownloadCalls     uint64																		// 统计用？需要
	//atomicErroredCalls      uint64
	//atomicFormContractCalls uint64
	//atomicRenewCalls        uint64
	//atomicReviseCalls       uint64
	//atomicSettingsCalls     uint64
	//atomicUnrecognizedCalls uint64
	//
	//// Error management. There are a few different types of errors returned by
	//// the host. These errors intentionally not persistent, so that the logging
	//// limits of each error type will be reset each time the host is reset.
	//// These values are not persistent.
	// atomicCommunicationErrors uint64																		// errors
	// atomicConnectionErrors    uint64
	// atomicConsensusErrors     uint64
	// atomicInternalErrors      uint64
	// atomicNormalErrors        uint64
	//
	//// Dependencies.
	//cs           modules.ConsensusSet																		//
	//g            modules.Gateway																			//
	//tpool        modules.TransactionPool																	//  以太坊都有？
	//wallet       modules.Wallet																			//
	//dependencies modules.Dependencies																		//
	//modules.StorageManager
	//
	//// Host ACID fields - these fields need to be updated in serial, ACID
	//// transactions.
	//announced         bool																					// 初始化从什么地方读取? jason还是db还是和以太坊一样的config文件？
	//announceConfirmed bool																					// confirm啥东西？protocol ？
	//blockHeight       types.BlockHeight																		// 需要? 可以从backend得到
	//publicKey         types.SiaPublicKey																		// wallet可以找到？
	//secretKey         crypto.SecretKey																		// wallet可以找到？
	//recentChange      modules.ConsensusChangeID																// 用处？
	//unlockHash        types.UnlockHash // A wallet address that can receive coins.							// 这是啥？
	//
	//// Host transient fields - these fields are either determined at startup or
	//// otherwise are not critical to always be correct.
	//autoAddress          modules.NetAddress // Determined using automatic tooling in network.go				// 网络IP?
	//financialMetrics     modules.HostFinancialMetrics															// ？
	//settings             modules.HostInternalSettings															// protocol相关？setting写入json还是db？
	//revisionNumber       uint64																				// protocol相关? 存储什么地方 ？
	//workingStatus        modules.HostWorkingStatus															// network: 与以太坊server一体
	//connectabilityStatus modules.HostConnectabilityStatus														// 与以太坊server handler一体
	//
	//// A map of storage obligations that are currently being modified. Locks on
	//// storage obligations can be long-running, and each storage obligation can
	//// be locked separately.
	//lockedStorageObligations map[types.FileContractID]*siasync.TryMutex										// jacky
	//
	//// Utilities.
	//db         *persist.BoltDatabase																			// 记录什么数据 ? ?
	//listener   net.Listener																					// 与以太坊server handler一体
	//log        *persist.Logger																				自带log15
	//mu         sync.RWMutex
	//persistDir string																							//记录什么数据 ？？
	//port       string
	//tg         siasync.ThreadGroup  																			// below tg

	tg			gdxthreads.ThreadManager
	storageManager.StorageManager
}