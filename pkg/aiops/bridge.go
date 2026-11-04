package aiops

import (
	"PromAI/pkg/database"
	"PromAI/pkg/eventhub"
)

func BridgeAlertInstance(inst database.AlertInstance) {
	if database.DB == nil {
		return
	}
	h := eventhub.New(database.DB)
	_ = h.CreateFromAlert(inst)
}
