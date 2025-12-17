package validator

import (
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
)

// IsContractCreation определяет, является ли транзакция созданием контракта
// Создание контракта: To == nil && Data != nil && len(Data) > 0
func IsContractCreation(tx types.GTransaction) bool {
	return tx.To() == nil && len(tx.Data()) > 0
}

// IsContractCall определяет, является ли транзакция вызовом контракта
// Вызов контракта: To != nil && (HasContractCode || len(Data) > 0)
func IsContractCall(tx types.GTransaction) bool {
	if tx.To() == nil {
		return false
	}

	// Если есть данные, это может быть вызов контракта
	if len(tx.Data()) > 0 {
		// Проверяем, есть ли код контракта по адресу
		localVault := storage.GetVault()
		if localVault.HasContractCode(*tx.To()) {
			return true
		}
		// Если данных нет, но есть адрес с кодом - это тоже вызов
		return false
	}

	// Если данных нет, но адрес имеет код - это вызов контракта
	localVault := storage.GetVault()
	return localVault.HasContractCode(*tx.To())
}

// IsContractTransaction определяет, является ли транзакция контрактной
// (создание или вызов контракта)
func IsContractTransaction(tx types.GTransaction) bool {
	return IsContractCreation(tx) || IsContractCall(tx)
}

// ShouldExecuteVM определяет, нужно ли выполнять VM для транзакции
// VM выполняется если:
// 1. Транзакция типа AppTxType (явно указано)
// 2. Или это создание контракта (To == nil && Data != nil)
// 3. Или это вызов контракта (To != nil && HasContractCode)
func ShouldExecuteVM(tx types.GTransaction) bool {
	// Если тип транзакции AppTxType - это контрактная транзакция
	if tx.Type() == types.AppTxType {
		return true
	}

	// Проверяем по содержимому транзакции
	return IsContractTransaction(tx)
}
