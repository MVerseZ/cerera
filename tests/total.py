#!/usr/bin/env python3
"""
Test script for cerera.account.getAll method
"""

import requests
import json
import sys

def test_account_get_all():
    """Test the cerera.account.getAll method"""
    
    # Default endpoint - adjust if needed
    url = "http://localhost:1337/app"
    
    # Request payload
    payload = {
        "method": "cerera.account.getAll",
        "params": [],
        "id": 11
    }
    
    # Headers
    headers = {
        "Content-Type": "application/json"
    }
    
    try:
        print(f"Sending request to {url}")
        print(f"Payload: {json.dumps(payload, indent=2)}")
        
        # Make the request
        response = requests.post(url, json=payload, headers=headers, timeout=10)
        
        print(f"\nResponse Status: {response.status_code}")
        print(f"Response Headers: {dict(response.headers)}")
        
        # Try to parse JSON response
        try:
            response_data = response.json()
            print(f"Response Body: {json.dumps(response_data, indent=2)}")
            
            # Calculate total sum of all account balances
            if "result" in response_data and isinstance(response_data["result"], dict):
                total_balance = 0.0
                account_count = 0
                
                print(f"\n{'='*50}")
                print("ACCOUNT BALANCES:")
                print(f"{'='*50}")
                
                for account, balance in response_data["result"].items():
                    print(f"{account}: {balance}")
                    total_balance += float(balance)
                    account_count += 1
                
                print(f"{'='*50}")
                print(f"Total accounts: {account_count}")
                print(f"Total balance: {total_balance:,.2f}")
                print(f"{'='*50}")
            else:
                print("No account data found in response")
                
        except json.JSONDecodeError:
            print(f"Response Body (raw): {response.text}")
            
    except requests.exceptions.ConnectionError:
        print("Error: Could not connect to the server. Make sure the cerera node is running.")
        sys.exit(1)
    except requests.exceptions.Timeout:
        print("Error: Request timed out.")
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    test_account_get_all()
