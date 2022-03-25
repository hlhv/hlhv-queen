package conf

func GetKeyPath () string {
        return items.database.keyPath
}

func GetCertPath () string {
        return items.database.certPath
}

func GetConnKey () string {
        return items.database.connKey
}

func GetPortHlhv () int {
        return items.database.portHlhv
}

func GetPortHttps () int {
        return items.database.portHttps
}

func GetGardenFreq () int {
        return items.database.gardenFreq
}

func GetMaxBandAge () int {
        return items.database.maxBandAge
}

func GetTimeout () int {
        return items.database.timeout
}

func GetTimeoutReadHeader () int {
        return items.database.timeout
}

func GetTimeoutRead () int {
        return items.database.timeout
}

func GetTimeoutWrite () int {
        return items.database.timeoutWrite
}

func GetTimeoutIdle () int {
        return items.database.timeoutIdle
}
