#!/usr/bin/env python3
# -*- coding: utf-8 -*-

def validate_markdown_format(file_path):
    """验证Markdown文档格式"""
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    lines = content.split('\n')
    
    # 检查代码块配对
    code_block_starts = []
    code_block_ends = []
    
    for i, line in enumerate(lines):
        if line.strip() == '```':
            if not code_block_starts or len(code_block_starts) == len(code_block_ends):
                code_block_starts.append(i + 1)
            else:
                code_block_ends.append(i + 1)
    
    print(f"总行数: {len(lines)}")
    print(f"代码块开始标记数: {len(code_block_starts)}")
    print(f"代码块结束标记数: {len(code_block_ends)}")
    
    # 检查是否配对
    if len(code_block_starts) == len(code_block_ends):
        print("✅ 代码块标记配对正确")
    else:
        print("❌ 代码块标记不配对!")
        print(f"开始标记位置: {code_block_starts}")
        print(f"结束标记位置: {code_block_ends}")
    
    # 检查标题层级
    titles = []
    for i, line in enumerate(lines):
        if line.startswith('#'):
            level = len(line) - len(line.lstrip('#'))
            title_text = line.lstrip('# ').strip()
            titles.append((i + 1, level, title_text))
    
    print(f"\n标题层级检查:")
    for line_num, level, title in titles[:10]:  # 显示前10个标题
        print(f"  第{line_num}行: {'#' * level} {title}")
    
    if len(titles) > 10:
        print(f"  ... 还有 {len(titles) - 10} 个标题")
    
    return len(code_block_starts) == len(code_block_ends)

if __name__ == "__main__":
    file_path = r"D:\web\dgou\dgou-framework\README.md"
    is_valid = validate_markdown_format(file_path)
    print(f"\n文档格式验证结果: {'通过' if is_valid else '失败'}")