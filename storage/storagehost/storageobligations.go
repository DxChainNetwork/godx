// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehost

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/DxChainNetwork/godx/log"

	"github.com/DxChainNetwork/godx/core/vm"
	"github.com/DxChainNetwork/godx/ethdb"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
)

const (
	// resubmissionTimeout defines the number of blocks that a host will wait
	// before attempting to resubmit a transaction to the blockchain.
	// Typically, this transaction will contain either a file contract, a file
	// contract revision, or a storage proof.
	revisionSubmissionBuffer = uint64(144)
	resubmissionTimeout      = 3
)

const (
	obligationUnresolved storageObligationStatus = iota // Indicatees that an unitialized value was used. Unresolved
	// 表示使用了未初始化的值。
	obligationRejected // Indicates that the obligation never got started, no revenue gained or lost.
	// 表示义务从未开始，没有收入或损失。
	obligationSucceeded // Indicates that the obligation was completed, revenues were gained.
	// 表明义务已经完成，收入已经获得。
	obligationFailed // Indicates that the obligation failed, revenues and collateral were lost.
	// 表明义务失败，收入和抵押品丢失。
)

var (
	// errDuplicateStorageObligation is returned when the storage obligation
	// database already has a storage obligation with the provided file
	// contract. This error should only happen in the event of a developer
	// mistake.
	// 当存储义务数据库已经具有提供的文件合同的存储义务时，
	// 将返回errDuplicateStorageObligation。
	// 只有在开发人员出错时才会出现此错误

	// 存储义务具有与现有存储义务冲突的文件合同
	errDuplicateStorageObligation = errors.New("storage obligation has a file contract which conflicts with an existing storage obligation")

	// errInsaneFileContractOutputCounts is returned when a file contract has
	// the wrong number of outputs for either the valid or missed payouts.
	// 当文件合同的有效或错过的付款输出数量错误时，将返回errInsaneFileContractOutputCounts

	// 文件合同的有效或错过的付款的输出数量不正确
	errInsaneFileContractOutputCounts = errors.New("file contract has incorrect number of outputs for the valid or missed payouts")

	// errInsaneFileContractRevisionOutputCounts is returned when a file
	// contract has the wrong number of outputs for either the valid or missed
	// payouts.
	// 当文件合同的有效或错过的付款输出数量错误时，将返回errInsaneFileContractRevisionOutputCounts。

	// 文件合同修订的有效或错过的支出的输出数量不正确
	errInsaneFileContractRevisionOutputCounts = errors.New("file contract revision has incorrect number of outputs for the valid or missed payouts")

	// errInsaneOriginSetFileContract is returned is the final transaction of
	// the origin transaction set of a storage obligation does not have a file
	// contract in the final transaction - there should be a file contract
	// associated with every storage obligation.
	// 返回的errInsaneOriginSetFileContract是原始事务集的最终事务，
	// 存储义务在最终事务中没有文件契约 - 应该有与每个存储义务相关联的文件契约。

	//原始交易存储义务集应在最终交易中具有一个文件合同
	errInsaneOriginSetFileContract = errors.New("origin transaction set of storage obligation should have one file contract in the final transaction")

	// errInsaneOriginSetSize is returned if the origin transaction set of a
	// storage obligation is empty - there should be a file contract associated
	// with every storage obligation.
	// 如果存储义务的原始事务集为空，则返回errInsaneOriginSetSize  - 应该存在与每个存储义务关联的文件协定。

	//原始交易存储义务集的大小为零
	errInsaneOriginSetSize = errors.New("origin transaction set of storage obligation is size zero")

	// errInsaneRevisionSetRevisionCount is returned if the final transaction
	// in the revision transaction set of a storage obligation has more or less
	// than one file contract revision.
	// 如果存储义务的修订事务集中的最终事务具有多于或少于一个文件合同修订，则返回错误信号恢复设置。

	//修订交易存储义务集应在最终交易中有一个文件合同修订
	errInsaneRevisionSetRevisionCount = errors.New("revision transaction set of storage obligation should have one file contract revision in the final transaction")

	// errInsaneStorageObligationRevision is returned if there is an attempted
	// storage obligation revision which does not have sensical inputs.
	// 如果尝试存储义务修订版没有敏感输入，则返回errInsaneStorageObligationRevision。

	//存储义务的修订没有意义
	errInsaneStorageObligationRevision = errors.New("revision to storage obligation does not make sense")

	// errInsaneStorageObligationRevisionData is returned if there is an
	// attempted storage obligation revision which does not have sensical
	// inputs.
	//如果尝试存储义务修订版没有敏感输入，则返回错误信息。
	errInsaneStorageObligationRevisionData = errors.New("revision to storage obligation has insane data")

	// errNoBuffer is returned if there is an attempted storage obligation that
	// needs to have the storage proof submitted in less than
	// revisionSubmissionBuffer blocks.
	// 如果存在尝试存储义务需要在少于revisionSubmissionBuffer块中提交存储证明，则返回errNoBuffer。

	//文件合同被拒绝，因为存储证明窗口太近了
	errNoBuffer = errors.New("file contract rejected because storage proof window is too close")

	// errNoStorageObligation is returned if the requested storage obligation
	// is not found in the database. 如果在数据库中找不到请求的存储义务，则返回errNoStorageObligation。

	//在数据库中找不到存储义务
	errNoStorageObligation = errors.New("storage obligation not found in database")

	// errObligationUnlocked is returned when a storage obligation is being
	// removed from lock, but is already unlocked. 当存储义务从锁定中删除但已解锁时，将返回errObligationUnlocked。

	//存储义务已解锁，不应解锁
	errObligationUnlocked = errors.New("storage obligation is unlocked, and should not be getting unlocked")
)

type (
	StorageObligation struct {
		// Storage obligations are broken up into ordered atomic sectors that are
		// exactly 4MiB each. By saving the roots of each sector, storage proofs
		// and modifications to the data can be made inexpensively by making use of
		// the merkletree.CachedTree. Sectors can be appended, modified, or deleted
		// and the host can recompute the Merkle root of the whole file without
		// much computational or I/O expense.
		//存储义务被分解为有序的原子扇区，每个扇区正好是4MiB。 通过保存每个扇区的根，
		// 可以通过使用merkletree.CachedTree以低成本方式进行存储证明和对数据的修改。
		// 可以附加，修改或删除扇区，主机可以重新计算整个文件的Merkle根，而无需太多的计算或I / O开销。
		SectorRoots       common.Hash
		StorageContractid common.Hash

		// Variables about the file contract that enforces the storage obligation.
		// The origin an revision transaction are stored as a set, where the set
		// contains potentially unconfirmed transactions.
		//关于强制执行存储义务的文件合同的变量。 原始修订事务存储为集合，其中集合包含可能未经证实的事务。
		ContractCost             *big.Int //合同花销
		LockedCollateral         *big.Int //锁定质押
		PotentialDownloadRevenue *big.Int //潜在的下载收入
		PotentialStorageRevenue  *big.Int //潜在的存储收入
		PotentialUploadRevenue   *big.Int //潜在的上传收入
		RiskedCollateral         *big.Int //风险质押
		TransactionFeesAdded     *big.Int

		// The negotiation height specifies the block height at which the file
		// contract was negotiated. If the origin transaction set is not accepted
		// onto the blockchain quickly enough, the contract is pruned from the
		// host. The origin and revision transaction set contain the contracts +
		// revisions as well as all parent transactions. The parents are necessary
		// because after a restart the transaction pool may be emptied out.
		// 协商高度指定协商文件合同的块高度。
		// 如果原始交易集未被足够快地接受到区块链上，则合同将从主机中删除。
		// 原始和修订事务集包含合同+修订以及所有父事务。 父母是必要的，因为重新启动后，事务池可能会被清空。
		NegotiationHeight uint64
		OriginStorage     []types.StorageContract
		Revision          []types.StorageContractRevision

		// Variables indicating whether the critical transactions in a storage
		// obligation have been confirmed on the blockchain.
		// 变量，指示是否已在区块链上确认存储义务中的关键事务。
		ObligationStatus    storageObligationStatus
		OriginConfirmed     bool
		ProofConfirmed      bool
		ProofConstructed    bool
		RevisionConfirmed   bool
		RevisionConstructed bool
	}

	storageObligationStatus uint64
)

func (i storageObligationStatus) String() string {
	if i == 0 {
		return "obligationUnresolved" // 未解决
	}
	if i == 1 {
		return "obligationRejected" // 被拒绝
	}
	if i == 2 {
		return "obligationSucceeded" // 成功
	}
	if i == 3 {
		return "obligationFailed" // 失败
	}
	return "storageObligationStatus(" + strconv.FormatInt(int64(i), 10) + ")"
}

// getStorageObligation fetches a storage obligation from the database
func getStorageObligation(db ethdb.Database, sc common.Hash) (StorageObligation, error) {
	so, errGet := vm.GetStorageObligation(db, sc)
	if errGet != nil {
		return StorageObligation{}, errGet
	}
	return so, nil
}

// putStorageObligation places a storage obligation into the database,
// overwriting the existing storage obligation if there is one.
func putStorageObligation(db ethdb.Database, so StorageObligation) error {
	err := vm.StoreStorageObligation(db, so.id(), so)
	if err != nil {
		return err
	}
	return nil
}

func deleteStorageObligation(db ethdb.Database, sc common.Hash) error {
	err := vm.DeleteStorageObligation(db, sc)
	if err != nil {
		return err
	}
	return nil
}

// expiration returns the height at which the storage obligation expires.
func (so StorageObligation) expiration() (number uint64) {
	if len(so.Revision) > 0 {
		return so.Revision[len(so.Revision)-1].NewWindowStart
	}
	return so.OriginStorage[0].WindowStart
}

// fileSize returns the size of the data protected by the obligation.
//返回受义务保护的数据大小
func (so StorageObligation) fileSize() uint64 {
	if len(so.Revision) > 0 {
		return so.Revision[len(so.Revision)-1].NewFileSize
	}
	return so.OriginStorage[0].FileSize
}

// id returns the id of the storage obligation, which is defined by the file
// contract id of the storage contract that governs the storage contract.
//返回这个存储义务的id，该ID由管理存储合同的文件合同的文件合同ID定义
func (so StorageObligation) id() (scid common.Hash) {
	return so.StorageContractid
}

// isSane checks that required assumptions about the storage obligation are
// correct.	检查所需的存储义务假设
func (so StorageObligation) isSane() error {
	if len(so.OriginStorage) == 0 {
		return errInsaneOriginSetSize
	}

	if len(so.Revision) == 0 {
		return errInsaneRevisionSetRevisionCount
	}

	return nil
}

// merkleRoot returns the file merkle root of a storage obligation.
//merkleRoot 返回关于存储义务的文件的merkle root
func (so StorageObligation) merkleRoot() common.Hash {
	if len(so.Revision) > 0 {
		return so.Revision[len(so.Revision)-1].NewFileMerkleRoot
	}
	return so.OriginStorage[len(so.OriginStorage)-1].FileMerkleRoot
}

// payouts returns the set of valid payouts and missed payouts that represent
// the latest revision for the storage obligation.
//返回有效支付和错过支付的集合，代表存储义务的最新Revision。
func (so StorageObligation) payouts() (validProofOutputs []types.DxcoinCharge, missedProofOutputs []types.DxcoinCharge) {
	if len(so.Revision) > 0 {
		validProofOutputs = so.Revision[len(so.Revision)-1].NewValidProofOutputs
		missedProofOutputs = so.Revision[len(so.Revision)-1].NewMissedProofOutputs
	}
	validProofOutputs = so.OriginStorage[len(so.OriginStorage)-1].ValidProofOutputs
	missedProofOutputs = so.OriginStorage[len(so.OriginStorage)-1].MissedProofOutputs
	return
}

// proofDeadline returns the height by which the storage proof must be
// submitted. 返回存储证明必须被提交的块的高度
func (so StorageObligation) proofDeadline() uint64 {
	if len(so.Revision) > 0 {
		return so.Revision[len(so.Revision)-1].NewWindowEnd
	}
	return so.OriginStorage[len(so.OriginStorage)-1].WindowEnd

}

// transactionID returns the ID of the transaction containing the Storage
// contract. 返回包含存储合约的交易的hash
func (so StorageObligation) transactionID() common.Hash {
	//TODO 通过存储合约ID获取到交易hash
	return common.Hash{}
}

//// value returns the value of fulfilling the storage obligation to the host.
////将履行存储义务的值返回给host
//func (so StorageObligation) value() *big.Int {
//	return nil
//}

// deleteStorageObligations deletes obligations from the database.
// It is assumed the deleted obligations don't belong in the database in the first place,
// so no financial metrics are updated.
// 删除存储义务从数据库中，假设已删除的义务首先不属于数据库，因此不会更新财务指标。
func (h StorageHost) deleteStorageObligations(db ethdb.Database, soids []common.Hash) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	for _, soid := range soids {
		err := deleteStorageObligation(db, soid)
		if err != nil {
			return err
		}
	}

	return nil
}

// queueActionItem adds an action item to the host at the input height so that
// the host knows to perform maintenance on the associated storage obligation
// when that height is reached.
// queueActionItem将操作项添加到输入高度的主机，以便主机知道在达到该高度时对相关存储义务执行维护
func (h *StorageHost) queueActionItem(height uint64, id common.Hash) error {

	if height < h.blockHeight {

	}
	return nil
}

// managedAddStorageObligation adds a storage obligation to the host. Because
// this operation can return errors, the transactions should not be submitted to
// the blockchain until after this function has indicated success. All of the
// sectors that are present in the storage obligation should already be on disk,
// which means that addStorageObligation should be exclusively called when
// creating a new, empty file contract or when renewing an existing file
// contract.	managedAddStorageObligation为主机添加存储义务。
// 由于此操作可以返回错误，因此在此函数指示成功之前，不应将事务提交到区块链。
// 存储义务中存在的所有扇区都应该已存在于磁盘上，
// 这意味着在创建新的空文件合同或续订现有文件合同时，应独占调用addStorageObligation。
func (h *StorageHost) managedAddStorageObligation(db ethdb.Database, so StorageObligation) error {
	err := func() error {
		h.lock.Lock()
		defer h.lock.Unlock()
		if _, ok := h.lockedStorageObligations[so.id()]; ok {
			log.Info("addStorageObligation called with an obligation that is not locked")
		}

		// Sanity check - There needs to be enough time left on the file contract
		// for the host to safely submit the file contract revision.
		// 完整性检查 - 文件合同需要有足够的时间让主机安全地提交文件合同修订版。
		if h.blockHeight+revisionSubmissionBuffer >= so.expiration() {
			return errNoBuffer
		}

		// Sanity check - the resubmission timeout needs to be smaller than storage
		// proof window.
		// 完整性检查 - 重新提交超时需要小于存储证明窗口。
		if so.expiration()+resubmissionTimeout >= so.proofDeadline() {
			//主机配置错误 - 存储证明窗口需要足够长，以便在需要时重新提交
			return errors.New("fill me in")
		}

		errDB := func() error {

			if len(so.SectorRoots) != 0 {

				//TODO	If the storage obligation already has sectors, it means that the
				//	file contract is being renewed, and that the sector should be
				// re-added with a new expiration height. If there is an error at any
				// point, all of the sectors should be removed.
				// 如果存储义务已经存在扇区，则表示正在续订文件合同，
				// 并且应该使用新的到期高度重新添加该扇区。 如果在任何时候出现错误，则应删除所有扇区。
			}

			errPut := vm.StoreStorageObligation(db, so.StorageContractid, so)
			if errPut != nil {
				return errPut
			}
			return nil

		}()

		if errDB != nil {
			return errDB
		}

		// Update the host financial metrics with regards to this storage
		// obligation.	更新有关此存储义务的主机财务指标。
		h.financialMetrics.ContractCount++
		h.financialMetrics.PotentialContractCompensation = *new(big.Int).Add(&h.financialMetrics.PotentialContractCompensation, so.ContractCost)
		h.financialMetrics.LockedStorageDeposit = *new(big.Int).Add(&h.financialMetrics.LockedStorageDeposit, so.LockedCollateral)
		h.financialMetrics.PotentialStorageRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialStorageRevenue, so.PotentialStorageRevenue)
		h.financialMetrics.PotentialDownloadBandwidthRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialDownloadBandwidthRevenue, so.PotentialDownloadRevenue)
		h.financialMetrics.PotentialUploadBandwidthRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialUploadBandwidthRevenue, so.PotentialUploadRevenue)
		h.financialMetrics.RiskedStorageDeposit = *new(big.Int).Add(&h.financialMetrics.RiskedStorageDeposit, so.RiskedCollateral)
		h.financialMetrics.TransactionFeeExpenses = *new(big.Int).Add(&h.financialMetrics.TransactionFeeExpenses, so.TransactionFeesAdded)

		return nil
	}()

	if err != nil {
		return err
	}

	//TODO Check that the transaction is fully valid and submit it to the
	// transaction pool.
	// 检查事务是否完全有效并将其提交到事务池。

	//TODO Queue the action items.
	// 排队执行项目。

	return nil
}

// modifyStorageObligation will take an updated storage obligation along with a
// list of sector changes and update the database to account for all of it. The
// sector modifications are only used to update the sector database, they will
// not be used to modify the storage obligation (most importantly, this means
// that sectorRoots needs to be updated by the calling function). Virtual
// sectors will be removed the number of times that they are listed, to remove
// multiple instances of the same virtual sector, the virtural sector will need
// to appear in 'sectorsRemoved' multiple times. Same with 'sectorsGained'.
// modifyStorageObligation将采用更新的存储义务以及扇区更改列表，并更新数据库以解决所有问题。
// 扇区修改仅用于更新扇区数据库，它们不会用于修改存储义务（最重要的是，这意味着需要通过调用函数更新sectorRoots）。
// 虚拟扇区将被删除它们列出的次数，为了删除同一虚拟扇区的多个实例，
// 虚拟扇区将需要多次出现在“sectorRemoved”中。 与'sectorGained'相同。
func (h *StorageHost) modifyStorageObligation(db ethdb.Database, so StorageObligation, sectorsRemoved []common.Hash, sectorsGained []common.Hash, gainedSectorData [][]byte) error {
	if _, ok := h.lockedStorageObligations[so.id()]; ok {
		log.Info("modifyStorageObligation called with an obligation that is not locked")
	}

	// Sanity check - there needs to be enough time to submit the file contract
	// revision to the blockchain.
	// 完整性检查 - 需要有足够的时间将文件合同修订提交到区块链。
	if so.expiration()-revisionSubmissionBuffer <= h.blockHeight {
		return errNoBuffer
	}

	// Sanity check - sectorsGained and gainedSectorData need to have the same length.
	// 完整性检查 -  sectorGained和obtainSectorData需要具有相同的长度。
	if len(sectorsGained) != len(gainedSectorData) {
		//用垃圾扇区数据修改修订版
		return errInsaneStorageObligationRevision
	}
	// Sanity check - all of the sector data should be modules.SectorSize
	// 所有扇区数据都应该是modules.SectorSize
	for _, data := range gainedSectorData {
		if uint64(len(data)) != uint64(1<<22) { //Sector Size	4 MiB
			return errInsaneStorageObligationRevision
		}
	}

	// TODO Note, for safe error handling, the operation order should be: add
	// sectors, update database, remove sectors. If the adding or update fails,
	// the added sectors should be removed and the storage obligation shoud be
	// considered invalid. If the removing fails, this is okay, it's ignored
	// and left to consistency checks and user actions to fix (will reduce host
	// capacity, but will not inhibit the host's ability to submit storage
	// proofs)
	// 注意，对于安全错误处理，操作顺序应该是：添加扇区，更新数据库，删除扇区。
	// 如果添加或更新失败，则应删除添加的扇区，并将存储义务视为无效。
	// 如果删除失败，这是可以的，它被忽略并留给一致性检查和用户操作来修复
	// （将减少主机容量，但不会抑制主机提交存储证明的能力）

	//var i int
	//var err error
	//for i = range sectorsGained {
	//	//err = h.AddSector(sectorsGained[i], gainedSectorData[i])
	//	if err != nil {
	//		break
	//	}
	//}
	//if err != nil {
	//	// Because there was an error, all of the sectors that got added need
	//	// to be reverted.
	//	// 因为存在错误，所有需要添加的扇区都需要还原。
	//	for j := 0; j < i; j++ {
	//		// Error is not checked because there's nothing useful that can be
	//		// done about an error.
	//		// 未检查错误，因为没有任何有用的错误信息。
	//		//_ = h.RemoveSector(sectorsGained[j])
	//	}
	//	return err
	//}

	var oldso StorageObligation
	var errOld error
	errDBso := func() error {

		oldso, errOld = getStorageObligation(db, so.id())
		if errOld != nil {
			return errOld
		}

		errOld = putStorageObligation(db, so)
		if errOld != nil {
			return errOld
		}
		return nil
	}()

	if errDBso != nil {
		//TODO
		// Because there was an error, all of the sectors that got added need
		// to be reverted.
		// 因为存在错误，所有添加的扇区都需要恢复。
		//for i := range sectorsGained {
		//	// Error is not checked because there's nothing useful that can be
		//	// done about an error.	未检查错误，因为没有任何关于错误的有用信息
		//	//_ = h.RemoveSector(sectorsGained[i])
		//}
		return errDBso
	}

	// TODO Call removeSector for all of the sectors that have been removed.
	// 所有扇区都应该被删除
	//for k := range sectorsRemoved {
	//	// Error is not checkeed because there's nothing useful that can be
	//	// done about an error. Failing to remove a sector is not a terrible
	//	// place to be, especially if the host can run consistency checks.
	//	// 错误并没有因为对错误没有任何帮助。 未能删除扇区并不是一个糟糕的地方，特别是如果主机可以运行一致性检查。
	//	_ = h.RemoveSector(sectorsRemoved[k])
	//}

	// Update the financial information for the storage obligation - apply the
	// new values.	更新存储义务的财务信息 - 应用新值。
	h.financialMetrics.PotentialContractCompensation = *new(big.Int).Add(&h.financialMetrics.PotentialContractCompensation, so.ContractCost)
	h.financialMetrics.LockedStorageDeposit = *new(big.Int).Add(&h.financialMetrics.LockedStorageDeposit, so.LockedCollateral)
	h.financialMetrics.PotentialStorageRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialStorageRevenue, so.PotentialStorageRevenue)
	h.financialMetrics.PotentialDownloadBandwidthRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialDownloadBandwidthRevenue, so.PotentialDownloadRevenue)
	h.financialMetrics.PotentialUploadBandwidthRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialUploadBandwidthRevenue, so.PotentialUploadRevenue)
	h.financialMetrics.RiskedStorageDeposit = *new(big.Int).Add(&h.financialMetrics.RiskedStorageDeposit, so.RiskedCollateral)
	h.financialMetrics.TransactionFeeExpenses = *new(big.Int).Add(&h.financialMetrics.TransactionFeeExpenses, so.TransactionFeesAdded)

	// Update the financial information for the storage obligation - remove the
	// old values.	更新存储义务的财务信息 - 删除旧值。
	h.financialMetrics.PotentialContractCompensation = *new(big.Int).Add(&h.financialMetrics.PotentialContractCompensation, oldso.ContractCost)
	h.financialMetrics.LockedStorageDeposit = *new(big.Int).Add(&h.financialMetrics.LockedStorageDeposit, oldso.LockedCollateral)
	h.financialMetrics.PotentialStorageRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialStorageRevenue, oldso.PotentialStorageRevenue)
	h.financialMetrics.PotentialDownloadBandwidthRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialDownloadBandwidthRevenue, oldso.PotentialDownloadRevenue)
	h.financialMetrics.PotentialUploadBandwidthRevenue = *new(big.Int).Add(&h.financialMetrics.PotentialUploadBandwidthRevenue, oldso.PotentialUploadRevenue)
	h.financialMetrics.RiskedStorageDeposit = *new(big.Int).Add(&h.financialMetrics.RiskedStorageDeposit, oldso.RiskedCollateral)
	h.financialMetrics.TransactionFeeExpenses = *new(big.Int).Add(&h.financialMetrics.TransactionFeeExpenses, oldso.TransactionFeesAdded)

	return nil

}

// PruneStaleStorageObligations will delete storage obligations from the host
// that, for whatever reason, did not make it on the block chain.
// As these stale storage obligations have an impact on the host financial metrics,
// this method updates the host financial metrics to show the correct values.
//将删除主机的存储义务，无论出于何种原因，它都不会在块链上进行
//由于这些陈旧的存储义务会对主机财务指标产生影响，因此此方法会更新主机财务指标以显示正确的值。
func (h *StorageHost) PruneStaleStorageObligations() error {
	return nil
}

// removeStorageObligation will remove a storage obligation from the host,
// either due to failure or success.
// removeStorageObligation将删除主机的存储义务，无论是由于失败还是成功。
func (h *StorageHost) removeStorageObligation(so StorageObligation, sos storageObligationStatus) error {
	return nil
}

func (h *StorageHost) resetFinancialMetrics() error {
	return nil
}

// threadedHandleActionItem will look at a storage obligation and determine
// which action is necessary for the storage obligation to succeed.	将考虑存储义务并确定存储义务成功所必需的行动
func (h *StorageHost) threadedHandleActionItem(soid common.Hash) {

}

// StorageObligations fetches the set of storage obligations in the host and
// returns metadata on them.	StorageObligations获取主机中的存储义务集并返回其上的元数据。
func (h *StorageHost) StorageObligations() (sos []StorageObligation) {
	return nil
}
