// 修改地形生成逻辑，添加性能优化
func (g *Game) generateTerrain(x, y int) {
    // 限制Y坐标范围，避免生成过高或过低的区块
    if y < -2 || y > 2 {
        return
    }
    
    // 根据玩家位置动态决定生成范围
    playerX := int(g.player.X)
    playerY := int(g.player.Y)
    
    // 只在玩家附近生成区块，减少计算量
    if abs(x-playerX) > 10 || abs(y-playerY) > 5 {
        return
    }
    
    // 使用噪声函数生成伪随机地形特征
    noiseValue := perlinNoise(float64(x), float64(y))
    
    // 根据噪声值确定地形类型
    terrainType := getTerrainType(noiseValue)
    
    // 在基础高度附近生成多层不同类型的方块
    baseHeight := int(noiseValue*5) + 1
    
    // 为每种地形类型定义符合其特征的方块分布模式
    for i := 0; i < 3; i++ {
        blockY := baseHeight - i
        if blockY >= 0 && blockY < g.worldHeight {
            g.world[x][blockY] = terrainType
        }
    }
}

// 添加高度限制避免生成过高或过低的区块
func (g *Game) updateWorld() {
    // 保持合理的可见区块加载范围
    for x := -10; x <= 10; x++ {
        for y := -2; y <= 2; y++ {
            // 只在必要时生成区块内容
            if g.world[x][y] == nil {
                g.generateTerrain(x, y)
            }
        }
    }
}