#!/usr/bin/env python3
"""
Простой тест для проверки faucet API
"""

import requests
import json
import random

def test_faucet():
    """Тестирует faucet API"""
    
    # URL API
    api_url = "http://localhost:1337/app"
    
    # Тестовый адрес
    test_address = "0x1234567890abcdef1234567890abcdef12345678"
    test_amount = 100.0
    
    # Создаем запрос
    data_req = {
        "method": "faucet",
        "jsonrpc": "2.0",
        "id": random.randint(1000, 9999),
        "params": [test_address, test_amount]
    }
    
    print(f"🚰 Тестируем faucet API...")
    print(f"Адрес: {test_address}")
    print(f"Сумма: {test_amount}")
    print(f"Запрос: {json.dumps(data_req, indent=2)}")
    
    try:
        response = requests.post(api_url, json=data_req, timeout=10)
        print(f"Статус ответа: {response.status_code}")
        
        if response.status_code == 200:
            result = response.json()
            print(f"Ответ: {json.dumps(result, indent=2)}")
            
            if 'result' in result:
                print(f"✅ Faucet успешен: {result['result']}")
                return True
            else:
                print(f"❌ Ошибка в ответе: {result}")
                return False
        else:
            print(f"❌ HTTP ошибка: {response.status_code} - {response.text}")
            return False
            
    except Exception as e:
        print(f"❌ Исключение: {e}")
        return False

if __name__ == "__main__":
    test_faucet()
