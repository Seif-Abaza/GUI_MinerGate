package main

import (
	"fmt"
	"minergate/internal/models"
)

// HelloWorldPlugin هو مثال بسيط لإضافة تظهر كيفية بناء الإضافة.
// يتم بناء هذه الإضافة كملف .so ويمكن تحميلها باستخدام internal/plugins/manager.go
// يشغل إجراءات بسيطة ويُظهر "Hello World" في السجل عند التهيئة.

type HelloWorldPlugin struct {
	initialized bool
}

// Ensure HelloWorldPlugin implements the PluginInterface.
// (The interface is in internal/plugins and is satisfied implicitly.)

func (p *HelloWorldPlugin) GetName() string {
	return "hello_world"
}

func (p *HelloWorldPlugin) GetVersion() string {
	return "0.1.0"
}

func (p *HelloWorldPlugin) GetDescription() string {
	return "A simple Hello World plugin for MinerGate Dashboard."
}

func (p *HelloWorldPlugin) Initialize(config map[string]interface{}) error {
	p.initialized = true
	fmt.Println("HelloWorldPlugin initialized")
	return nil
}

func (p *HelloWorldPlugin) OnMinerUpdate(miner *models.Miner) error {
	if !p.initialized {
		return fmt.Errorf("plugin not initialized")
	}
	return nil
}

func (p *HelloWorldPlugin) OnFarmUpdate(farm *models.Farm) error {
	if !p.initialized {
		return fmt.Errorf("plugin not initialized")
	}
	return nil
}

func (p *HelloWorldPlugin) OnAction(action string, minerID string) (bool, error) {
	if !p.initialized {
		return false, fmt.Errorf("plugin not initialized")
	}
	// Allow all actions.
	return true, nil
}

func (p *HelloWorldPlugin) Cleanup() error {
	p.initialized = false
	return nil
}

// Plugin is the exported symbol used by the plugin manager.
var Plugin HelloWorldPlugin
