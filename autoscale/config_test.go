package autoscale

import (
	"testing"
)

func TestYamlConfig(t *testing.T) {
	InitTestEnv()
	var data = `
compute_clusters:
  - id: 2v1zeQPnTgQ3a6U
    min_cores: 8
    max_cores: 32
    init_cores: 16
    window_seconds: 60
    autopause_seconds: 61
    cpu_lowerlimit: 0.4
    cpu_upperlimit: 0.8
    pd: 172.0.0.1
    region: 'us-west-3'
  - id: 345
  - id: 456
    region: 'hehe'
`
	defaultConfig := &YamlClusterConfig{
		MinCores: 4, MaxCores: 16, InitCores: 4, WindowSeconds: 120, AutoPauseSeconds: 121,
		CpuLowerLimit: 0.2, CpuUpperLimit: 0.7, Pd: "testpdhost"}
	testByte := []byte(data)
	yamlConfig := LoadYamlConfig(testByte, defaultConfig)
	assertEqual(t, len(yamlConfig.ComputeClusters), 3)
	assertEqual(t, yamlConfig.ComputeClusters[0].Id, "2v1zeQPnTgQ3a6U")
	assertEqual(t, yamlConfig.ComputeClusters[0].MinCores, 8)
	assertEqual(t, yamlConfig.ComputeClusters[0].MaxCores, 32)
	assertEqual(t, yamlConfig.ComputeClusters[0].InitCores, 16)
	assertEqual(t, yamlConfig.ComputeClusters[0].WindowSeconds, 60)
	assertEqual(t, yamlConfig.ComputeClusters[0].AutoPauseSeconds, 61)
	assertEqual(t, yamlConfig.ComputeClusters[0].CpuLowerLimit, 0.4)
	assertEqual(t, yamlConfig.ComputeClusters[0].CpuUpperLimit, 0.8)
	assertEqual(t, yamlConfig.ComputeClusters[0].Pd, "172.0.0.1")
	assertEqual(t, yamlConfig.ComputeClusters[0].Region, "us-west-3")

	assertEqual(t, yamlConfig.ComputeClusters[1].Id, "345")
	assertEqual(t, yamlConfig.ComputeClusters[1].MinCores, defaultConfig.MinCores)
	assertEqual(t, yamlConfig.ComputeClusters[1].MaxCores, defaultConfig.MaxCores)
	assertEqual(t, yamlConfig.ComputeClusters[1].InitCores, defaultConfig.InitCores)
	assertEqual(t, yamlConfig.ComputeClusters[1].WindowSeconds, defaultConfig.WindowSeconds)
	assertEqual(t, yamlConfig.ComputeClusters[1].AutoPauseSeconds, defaultConfig.AutoPauseSeconds)
	assertEqual(t, yamlConfig.ComputeClusters[1].CpuLowerLimit, defaultConfig.CpuLowerLimit)
	assertEqual(t, yamlConfig.ComputeClusters[1].CpuUpperLimit, defaultConfig.CpuUpperLimit)
	assertEqual(t, yamlConfig.ComputeClusters[1].Pd, defaultConfig.Pd)

	yamlConfig1 := yamlConfig.ValidConfig("123")
	assertEqual(t, len(yamlConfig1.ComputeClusters), 1)
	assertEqual(t, yamlConfig1.ComputeClusters[0].Id, "345")
	assertEqual(t, yamlConfig1.ComputeClusters[0].MinCores, defaultConfig.MinCores)
	assertEqual(t, yamlConfig1.ComputeClusters[0].MaxCores, defaultConfig.MaxCores)
	assertEqual(t, yamlConfig1.ComputeClusters[0].InitCores, defaultConfig.InitCores)
	assertEqual(t, yamlConfig1.ComputeClusters[0].WindowSeconds, defaultConfig.WindowSeconds)
	assertEqual(t, yamlConfig1.ComputeClusters[0].AutoPauseSeconds, defaultConfig.AutoPauseSeconds)
	assertEqual(t, yamlConfig1.ComputeClusters[0].CpuLowerLimit, defaultConfig.CpuLowerLimit)
	assertEqual(t, yamlConfig1.ComputeClusters[0].CpuUpperLimit, defaultConfig.CpuUpperLimit)
	assertEqual(t, yamlConfig1.ComputeClusters[0].Pd, defaultConfig.Pd)

	yamlConfig = yamlConfig.ValidConfig("us-west-3")
	assertEqual(t, len(yamlConfig.ComputeClusters), 2)
	assertEqual(t, yamlConfig.ComputeClusters[0].Id, "2v1zeQPnTgQ3a6U")
	assertEqual(t, yamlConfig.ComputeClusters[0].MinCores, 8)
	assertEqual(t, yamlConfig.ComputeClusters[0].MaxCores, 32)
	assertEqual(t, yamlConfig.ComputeClusters[0].InitCores, 16)
	assertEqual(t, yamlConfig.ComputeClusters[0].WindowSeconds, 60)
	assertEqual(t, yamlConfig.ComputeClusters[0].AutoPauseSeconds, 61)
	assertEqual(t, yamlConfig.ComputeClusters[0].CpuLowerLimit, 0.4)
	assertEqual(t, yamlConfig.ComputeClusters[0].CpuUpperLimit, 0.8)
	assertEqual(t, yamlConfig.ComputeClusters[0].Pd, "172.0.0.1")
	assertEqual(t, yamlConfig.ComputeClusters[0].Region, "us-west-3")
}

func TestYamlConfig2(t *testing.T) {
	InitTestEnv()
	var data = `
`
	defaultConfig := &YamlClusterConfig{
		MinCores: 4, MaxCores: 16, InitCores: 4, WindowSeconds: 120, AutoPauseSeconds: 121,
		CpuLowerLimit: 0.2, CpuUpperLimit: 0.7, Pd: "testpdhost"}
	testByte := []byte(data)
	yamlConfig := LoadYamlConfig(testByte, defaultConfig)
	assertEqual(t, len(yamlConfig.ComputeClusters), 0)
	data = `
compute_clusters:
  - id: 1379661944642684098
    min_cores: 16
    max_cores: 32
    init_cores: 16
    cpu_lowerlimit: 0.2
    cpu_upperlimit: 0.6
    region: us-west-2
    version: s3
    `

	defaultConfig = &YamlClusterConfig{
		MinCores: 4, MaxCores: 16, InitCores: 4, WindowSeconds: 120, AutoPauseSeconds: 121,
		CpuLowerLimit: 0.2, CpuUpperLimit: 0.7, Pd: "testpdhost"}
	testByte = []byte(data)
	yamlConfig = LoadYamlConfig(testByte, defaultConfig)
	assertEqual(t, len(yamlConfig.ComputeClusters), 1)
	assertEqual(t, yamlConfig.ComputeClusters[0].Id, "1379661944642684098")
	assertEqual(t, yamlConfig.ComputeClusters[0].MinCores, 16)
	assertEqual(t, yamlConfig.ComputeClusters[0].MaxCores, 32)
	assertEqual(t, yamlConfig.ComputeClusters[0].InitCores, 16)
	assertEqual(t, yamlConfig.ComputeClusters[0].WindowSeconds, 120)
	assertEqual(t, yamlConfig.ComputeClusters[0].AutoPauseSeconds, 121)
	assertEqual(t, yamlConfig.ComputeClusters[0].CpuLowerLimit, 0.2)
	assertEqual(t, yamlConfig.ComputeClusters[0].CpuUpperLimit, 0.6)
	assertEqual(t, yamlConfig.ComputeClusters[0].Pd, "testpdhost")
	assertEqual(t, yamlConfig.ComputeClusters[0].Region, "us-west-2")
	assertEqual(t, yamlConfig.ComputeClusters[0].Version, "s3")
}
