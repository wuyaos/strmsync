#!/usr/bin/env bash
# 统一package名称

set -euo pipefail

BASE="/mnt/c/Users/wff19/Desktop/strm/backend"

echo "=== 统一package名称 ==="

# core/目录 -> package core
echo "统一 core/ package名..."
find "$BASE/core" -name "*.go" -exec sed -i 's/^package config$/package core/g; s/^package database$/package core/g' {} \;

# service/目录 -> package service
echo "统一 service/ package名..."
find "$BASE/service" -name "*.go" -exec sed -i 's/^package types$/package service/g; s/^package job$/package service/g; s/^package taskrun$/package service/g; s/^package planner$/package service/g; s/^package executor$/package service/g; s/^package strm$/package service/g; s/^package filemonitor$/package service/g' {} \;

# dataserver/目录 -> package dataserver (排除proto子目录)
echo "统一 dataserver/ package名..."
find "$BASE/dataserver" -maxdepth 1 -name "*.go" -exec sed -i 's/^package clouddrive2$/package dataserver/g' {} \;

# mediaserver/目录 -> package mediaserver
echo "统一 mediaserver/ package名..."
find "$BASE/mediaserver" -name "*.go" -exec sed -i 's/^package client$/package mediaserver/g' {} \;

# handler/目录已经统一是handler，无需修改

echo "✓ package名称统一完成"
