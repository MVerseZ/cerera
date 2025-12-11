#!/usr/bin/env python3
"""
Скрипт для подключения к SMB папке с учетными данными и сохранения файла
Автоматически устанавливает необходимые зависимости при первом запуске.

Примеры использования:

1. Базовое использование:
   python test_smb_upload.py --server 192.168.1.100 --share shared --username user --password pass

2. С указанием домена:
   python test_smb_upload.py --server 192.168.1.100 --share shared --username user --password pass --domain WORKGROUP

3. С указанием пути внутри шары:
   python test_smb_upload.py --server 192.168.1.100 --share shared --username user --password pass --remote-path folder/subfolder

4. С указанием локального файла:
   python test_smb_upload.py --server 192.168.1.100 --share shared --username user --password pass --local-file myfile.txt
"""

import os
import sys
import subprocess
from datetime import datetime

# Автоматическая установка зависимостей
def install_requirements():
    """Устанавливает необходимые зависимости, если они отсутствуют"""
    try:
        import smbclient
        return True
    except ImportError:
        print("📦 Установка зависимостей (smbprotocol)...")
        try:
            subprocess.check_call([sys.executable, "-m", "pip", "install", "smbprotocol>=1.10.0", "--quiet"])
            print("✅ Зависимости успешно установлены")
            return True
        except subprocess.CalledProcessError as e:
            print(f"❌ Ошибка при установке зависимостей: {e}")
            print("Попробуйте установить вручную: pip install smbprotocol")
            return False

# Проверяем и устанавливаем зависимости
if not install_requirements():
    sys.exit(1)

# Импортируем после проверки зависимостей
from smbclient import open_file, register_session, remove_session
from smbclient.path import exists, makedirs


def connect_and_save_file(
    server: str,
    share: str,
    username: str,
    password: str,
    domain: str = "",
    remote_path: str = "",
    local_file_path: str = "test_file.txt",
    file_content: str = "Test file content from cerera tests"
):
    """
    Подключается к SMB папке и сохраняет файл
    
    Args:
        server: IP адрес или имя SMB сервера
        share: Имя шары (например, 'shared' или 'C$')
        username: Имя пользователя
        password: Пароль
        domain: Домен (опционально, по умолчанию пустая строка)
        remote_path: Путь внутри шары (например, 'folder/subfolder')
        local_file_path: Путь к локальному файлу для сохранения
        file_content: Содержимое файла (если файл не существует, будет создан)
    """
    smb_path = f"\\\\{server}\\{share}"
    if remote_path:
        smb_path = f"{smb_path}\\{remote_path}"
    
    print(f"🔌 Подключение к SMB папке...")
    print(f"Сервер: {server}")
    print(f"Шара: {share}")
    print(f"Пользователь: {username}")
    print(f"Домен: {domain if domain else '(не указан)'}")
    print(f"Путь: {smb_path}")
    
    try:
        # Регистрируем сессию с учетными данными
        register_session(
            server,
            username=username,
            password=password,
            domain=domain if domain else None
        )
        print(f"✅ Сессия успешно зарегистрирована")
        
        # Проверяем существование пути
        if remote_path:
            remote_dir = f"\\\\{server}\\{share}\\{remote_path}"
            if not exists(remote_dir):
                print(f"📁 Создание директории: {remote_dir}")
                makedirs(remote_dir, exist_ok=True)
        
        # Формируем полный путь к файлу на SMB
        remote_file_path = f"{smb_path}\\{os.path.basename(local_file_path)}"
        
        # Проверяем существование локального файла
        if not os.path.exists(local_file_path):
            print(f"📝 Создание локального файла: {local_file_path}")
            with open(local_file_path, 'w', encoding='utf-8') as f:
                f.write(file_content)
        
        # Читаем содержимое локального файла
        print(f"📖 Чтение локального файла: {local_file_path}")
        with open(local_file_path, 'rb') as f:
            file_data = f.read()
        
        # Сохраняем файл на SMB папку
        print(f"💾 Сохранение файла на SMB: {remote_file_path}")
        with open_file(remote_file_path, mode='wb') as smb_file:
            smb_file.write(file_data)
        
        print(f"✅ Файл успешно сохранен: {remote_file_path}")
        
        # Проверяем, что файл действительно сохранен
        if exists(remote_file_path):
            print(f"✅ Подтверждение: файл существует на SMB папке")
            
            # Читаем файл обратно для проверки
            with open_file(remote_file_path, mode='rb') as smb_file:
                read_data = smb_file.read()
            
            if read_data == file_data:
                print(f"✅ Проверка: содержимое файла совпадает")
            else:
                print(f"⚠️  Предупреждение: содержимое файла не совпадает")
        else:
            print(f"❌ Ошибка: файл не найден после сохранения")
            return False
        
        return True
        
    except Exception as e:
        print(f"❌ Ошибка при работе с SMB: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    finally:
        # Закрываем сессию
        try:
            remove_session(server)
            print(f"🔌 Сессия закрыта")
        except:
            pass


def main():
    """Основная функция для запуска скрипта"""
    import argparse
    
    parser = argparse.ArgumentParser(
        description='Подключение к SMB папке и сохранение файла'
    )
    parser.add_argument('--server', required=True, help='IP адрес или имя SMB сервера')
    parser.add_argument('--share', required=True, help='Имя шары (например, shared)')
    parser.add_argument('--username', required=True, help='Имя пользователя')
    parser.add_argument('--password', required=True, help='Пароль')
    parser.add_argument('--domain', default='', help='Домен (опционально)')
    parser.add_argument('--remote-path', default='', help='Путь внутри шары (опционально)')
    parser.add_argument('--local-file', default='test_file.txt', help='Путь к локальному файлу')
    parser.add_argument('--content', default='', help='Содержимое файла (если файл не существует)')
    
    args = parser.parse_args()
    
    # Если содержимое не указано, используем значение по умолчанию
    file_content = args.content if args.content else f"Test file content from cerera tests\nCreated at: {datetime.now()}"
    
    success = connect_and_save_file(
        server=args.server,
        share=args.share,
        username=args.username,
        password=args.password,
        domain=args.domain,
        remote_path=args.remote_path,
        local_file_path=args.local_file,
        file_content=file_content
    )
    
    if success:
        print(f"\n✅ Тест успешно завершен!")
        sys.exit(0)
    else:
        print(f"\n❌ Тест завершился с ошибкой")
        sys.exit(1)


if __name__ == "__main__":
    main()

