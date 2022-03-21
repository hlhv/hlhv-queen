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

func GetTimeout () int {
        return items.database.timeout
}

func GetPortHlhv () int {
        return items.database.portHlhv
}

func GetPortHttps () int {
        return items.database.portHttps
}
