#!/bin/bash

# LLMIO 性能测试脚本
# 用于验证优化效果

set -e

echo "========================================="
echo "LLMIO 性能测试"
echo "========================================="

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查依赖
check_dependencies() {
    echo -e "\n${YELLOW}检查依赖工具...${NC}"
    
    commands=("curl" "ab" "docker")
    for cmd in "${commands[@]}"; do
        if command -v $cmd &> /dev/null; then
            echo -e "${GREEN}✓${NC} $cmd 已安装"
        else
            echo -e "${RED}✗${NC} $cmd 未安装"
            exit 1
        fi
    done
}

# 测试API响应时间
test_response_time() {
    echo -e "\n${YELLOW}测试API响应时间...${NC}"
    
    # 测试模型列表接口
    response_time=$(curl -o /dev/null -s -w '%{time_total}' http://localhost:7070/v1/models)
    echo -e "模型列表接口响应时间: ${GREEN}${response_time}s${NC}"
    
    # 判断是否小于阈值 (0.1秒)
    if (( $(echo "$response_time < 0.1" | bc -l) )); then
        echo -e "${GREEN}✓${NC} 响应时间优秀"
    else
        echo -e "${YELLOW}!${NC} 响应时间可以优化"
    fi
}

# 并发测试
test_concurrency() {
    echo -e "\n${YELLOW}测试并发性能...${NC}"
    
    # 使用 Apache Bench 进行并发测试
    echo "运行 100 个请求，并发 10..."
    ab -n 100 -c 10 -q http://localhost:7070/v1/models > /tmp/ab_result.txt 2>&1
    
    # 提取关键指标
    requests_per_sec=$(grep "Requests per second" /tmp/ab_result.txt | awk '{print $4}')
    time_per_request=$(grep "Time per request" /tmp/ab_result.txt | head -1 | awk '{print $4}')
    
    echo -e "每秒请求数: ${GREEN}${requests_per_sec}${NC} req/s"
    echo -e "平均响应时间: ${GREEN}${time_per_request}${NC} ms"
    
    # 判断性能
    if (( $(echo "$requests_per_sec > 100" | bc -l) )); then
        echo -e "${GREEN}✓${NC} 并发性能优秀"
    else
        echo -e "${YELLOW}!${NC} 并发性能需要优化"
    fi
}

# 测试Docker镜像大小
test_docker_image_size() {
    echo -e "\n${YELLOW}检查Docker镜像大小...${NC}"
    
    if docker images | grep -q "llmio"; then
        size=$(docker images llmio --format "{{.Size}}")
        echo -e "镜像大小: ${GREEN}${size}${NC}"
        
        # 提取数字部分进行比较
        size_mb=$(echo $size | sed 's/MB//' | sed 's/GB/*1024/' | bc)
        
        if (( $(echo "$size_mb < 300" | bc -l) )); then
            echo -e "${GREEN}✓${NC} 镜像大小优秀 (< 300MB)"
        else
            echo -e "${YELLOW}!${NC} 镜像大小可以优化"
        fi
    else
        echo -e "${YELLOW}!${NC} 未找到 llmio 镜像"
    fi
}

# 测试缓存性能
test_cache_performance() {
    echo -e "\n${YELLOW}测试缓存性能...${NC}"
    
    echo "首次请求（冷缓存）..."
    time1=$(curl -o /dev/null -s -w '%{time_total}' http://localhost:7070/v1/models)
    echo -e "响应时间: ${GREEN}${time1}s${NC}"
    
    sleep 1
    
    echo "第二次请求（热缓存）..."
    time2=$(curl -o /dev/null -s -w '%{time_total}' http://localhost:7070/v1/models)
    echo -e "响应时间: ${GREEN}${time2}s${NC}"
    
    # 计算缓存带来的性能提升
    improvement=$(echo "scale=2; ($time1 - $time2) / $time1 * 100" | bc)
    echo -e "缓存性能提升: ${GREEN}${improvement}%${NC}"
}

# 内存使用测试
test_memory_usage() {
    echo -e "\n${YELLOW}检查内存使用...${NC}"
    
    if docker ps | grep -q "llmio"; then
        container_id=$(docker ps | grep llmio | awk '{print $1}')
        mem_usage=$(docker stats $container_id --no-stream --format "{{.MemUsage}}")
        echo -e "内存使用: ${GREEN}${mem_usage}${NC}"
    else
        echo -e "${YELLOW}!${NC} 容器未运行"
    fi
}

# 生成测试报告
generate_report() {
    echo -e "\n========================================="
    echo -e "${GREEN}测试完成！${NC}"
    echo -e "========================================="
    echo ""
    echo "详细报告已保存至: /tmp/ab_result.txt"
    echo ""
    echo "优化建议："
    echo "1. 如果响应时间 > 100ms，考虑增加缓存"
    echo "2. 如果并发性能 < 100 req/s，考虑优化数据库查询"
    echo "3. 如果镜像大小 > 300MB，考虑使用 Alpine 基础镜像"
    echo "4. 定期监控内存使用，避免内存泄漏"
}

# 主函数
main() {
    check_dependencies
    test_docker_image_size
    
    # 检查服务是否运行
    if ! curl -s http://localhost:7070/v1/models > /dev/null 2>&1; then
        echo -e "\n${RED}错误: LLMIO 服务未运行${NC}"
        echo "请先启动服务: docker-compose up -d"
        exit 1
    fi
    
    test_response_time
    test_cache_performance
    test_concurrency
    test_memory_usage
    generate_report
}

# 运行测试
main