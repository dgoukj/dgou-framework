#!/usr/bin/env python3
# -*- coding: utf-8 -*-

def simple_validate(file_path):
    """简单的文档格式验证"""
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    lines = content.split('\n')
    
    # 统计代码块
    code_blocks = 0
    for line in lines:
        if line.strip() == '```':
            code_blocks += 1
    
    print(f"文档总行数: {len(lines)}")
    print(f"代码块标记对数: {code_blocks // 2}")
    print(f"剩余未配对标记: {code_blocks % 2}")
    
    if code_blocks % 2 == 0:
        print("✅ 文档格式基本正确")
        return True
    else:
        print("❌ 存在未配对的代码块标记")
        return False

if __name__ == "__main__":
    file_path = r"D:\web\dgou\dgou-framework\README.md"
    simple_validate(file_path)