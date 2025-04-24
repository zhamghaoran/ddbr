#!/bin/bash

# 创建data目录（如果不存在）
mkdir -p ./ddbr-server/data
mkdir -p ./ddbr-server/data-slave

# 创建初始化数据文件
cat > ./ddbr-server/data/raft_state.json << 'EOF'
{
  "current_term": 1,
  "voted_for": -1,
  "logs": [
    {
      "term": 1,
      "index": 1,
      "command": "set:init_key1:初始化数据1"
    },
    {
      "term": 1,
      "index": 2,
      "command": "set:init_key2:初始化数据2"
    },
    {
      "term": 1,
      "index": 3,
      "command": "set:config:系统配置信息"
    }
  ]
}
EOF

echo "初始数据文件已生成到 ./ddbr-server/data/raft_state.json" 