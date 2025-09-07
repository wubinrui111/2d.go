package main
import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// 游戏屏幕和玩家常量定义
const (
	ScreenWidth  = 640
	ScreenHeight = 480
	PlayerSize   = 50
	WorldWidth   = 2000  // 虚拟游戏世界宽度
	WorldHeight  = 2000  // 虚拟游戏世界高度
	PlayerSpeed  = 4.0   // 玩家移动速度
	CameraLerp   = 0.1   // 摄角机跟随速度 (0.01 ~ 0.3，越小越慢越平滑)
	
	// 物理常量定义玩家重力和跳跃行为
	Gravity       = 0.5
	JumpPower     = 12.0
	PlayerMaxFall = 10.0
	
	// 地形生成常量
	BlockSize     = 50
	ChunkSize     = 10              // 每个区块的方块数
	ChunkWorldSize = BlockSize * ChunkSize // 每个区块的世界尺寸
	GenerationDistance = 3          // 生成距离（以区块为单位）
	UndergroundDepth   = 10         // 地下深度
	
	// 游戏模式枚举
	GameModeCreative = iota // 创造模式
	GameModeSurvival        // 生存模式
	
	// 生存模式参数
	MaxPlaceDistance = 5 * BlockSize
)

// ItemType 定义游戏中可用的方块类型
type ItemType int

// 方块类型常量定义
const (
	ItemTypeGrass ItemType = iota // 草地
	ItemTypeDirt                  // 泥土
	ItemTypeStone                 // 石头
	ItemTypeSand                  // 沙子
	ItemTypeWood                  // 木头
	ItemTypeWater                 // 水
	ItemTypeLava                  // 岩浆
	ItemTypeSnow                  // 雪
)

// Item 定义游戏中可用的物品结构
type Item struct {
	Type        ItemType
	Name        string
	Color       color.RGBA
	Description string
}

// 全局物品注册表，包含所有可用方块类型及其属性
var itemRegistry = map[ItemType]Item{
	ItemTypeGrass: {
		Type:        ItemTypeGrass,
		Name:        "Grass",
		Color:       color.RGBA{50, 180, 50, 255},
		Description: "Green grass block",
	},
	ItemTypeDirt: {
		Type:        ItemTypeDirt,
		Name:        "Dirt",
		Color:       color.RGBA{150, 100, 50, 255},
		Description: "Brown dirt block",
	},
	ItemTypeStone: {
		Type:        ItemTypeStone,
		Name:        "Stone",
		Color:       color.RGBA{100, 100, 100, 255},
		Description: "Gray stone block",
	},
	ItemTypeSand: {
		Type:        ItemTypeSand,
		Name:        "Sand",
		Color:       color.RGBA{255, 220, 100, 255},
		Description: "Golden sand block",
	},
	ItemTypeWood: {
		Type:        ItemTypeWood,
		Name:        "Wood",
		Color:       color.RGBA{150, 100, 50, 255},
		Description: "Brown wood block",
	},
	ItemTypeWater: {
		Type:        ItemTypeWater,
		Name:        "Water",
		Color:       color.RGBA{50, 100, 255, 200},
		Description: "Blue water block",
	},
	ItemTypeLava: {
		Type:        ItemTypeLava,
		Name:        "Lava",
		Color:       color.RGBA{255, 100, 0, 200},
		Description: "Hot lava block",
	},
	ItemTypeSnow: {
		Type:        ItemTypeSnow,
		Name:        "Snow",
		Color:       color.RGBA{230, 230, 255, 255},
		Description: "White snow block",
	},
}

// TerrainType 定义地形类型枚举
type TerrainType int

// 地形类型常量定义
const (
	TerrainTypePlains TerrainType = iota // 平原
	TerrainTypeHills                     // 丘陵
	TerrainTypeMountains                 // 山脉
	TerrainTypeDesert                    // 沙漠
	TerrainTypeForest                    // 森林
	TerrainTypeSnowyPlains               // 雪原
	TerrainTypeSwamp                     // 沼泽
	TerrainTypeJungle                    // 丛林
	TerrainTypeTaiga                     // 针叶林
	TerrainTypeSavanna                   // 热带草原
	TerrainTypeCanyon                    // 峡谷
)

// Block 定义游戏中的方块结构
type Block struct {
	X, Y, W, H float64
	Type       ItemType // 方块类型
}

// Chunk 定义地形区块结构
type Chunk struct {
	X, Y   int
	Blocks []Block
}

// Game 定义游戏主结构，包含所有游戏状态
type Game struct {
	playerX, playerY   float64 // 玩家在世界中的位置
	playerVelocityY    float64 // 玩家垂直速度
	playerOnGround     bool    // 玩家是否在地面上

	// 实际摄像头偏移（用于绘制）
	cameraX, cameraY float64
	
	// 地面方块列表
	blocks []Block
	
	// 区块管理
	chunks map[string]*Chunk
	
	// 世界边界（用于地下世界）
	worldMinX, worldMaxX float64
	worldMinY, worldMaxY float64
	
	// 游戏模式
	gameMode int
	
	// 框选相关字段
	selecting          bool    // 是否正在框选
	selectionStartX    float64 // 框选起始点X坐标
	selectionStartY    float64 // 框选起始点Y坐标
	selectionEndX      float64 // 框选结束点X坐标
	selectionEndY      float64 // 框选结束点Y坐标
	
	// 选中块相关字段
	selectedBlockX     float64 // 选中方块的X坐标
	selectedBlockY     float64 // 选中方块的Y坐标
	hasSelectedBlock   bool    // 是否有选中的方块
	
	// 当前选中的物品类型
	currentItemType ItemType
	
	// 物品栏相关
	hotbarSelected int // 当前选中物品栏位置 (0-2)
}

// getMouseWorldPosition 获取鼠标在世界坐标系中的位置
func (g *Game) getMouseWorldPosition() (float64, float64) {
	x, y := ebiten.CursorPosition()
	worldX := float64(x) - g.cameraX
	worldY := float64(y) - g.cameraY
	return worldX, worldY
}

// getBlockCoordinate 将世界坐标转换为方块坐标
func getBlockCoordinate(worldCoord float64) float64 {
	return math.Floor(worldCoord/BlockSize) * BlockSize
}

// isBlockAt 检查指定位置是否有方块
func (g *Game) isBlockAt(x, y float64) bool {
	for _, block := range g.blocks {
		// 精确比较方块的X和Y坐标，确保匹配指定位置
		if block.X == x && block.Y == y {
			return true
		}
	}
	return false
}

// hasLineOfSight 检查指定位置和玩家之间是否有视线（用于创造模式）
func (g *Game) hasLineOfSight(blockX, blockY, playerX, playerY float64) bool {
	// 在创造模式下，总是有视线
	if g.gameMode == GameModeCreative {
		return true
	}
	
	// 计算玩家中心位置
	playerCenterX := playerX + PlayerSize/2
	playerCenterY := playerY + PlayerSize/2
	
	// 计算方块中心位置
	blockCenterX := blockX + BlockSize/2
	blockCenterY := blockY + BlockSize/2
	
	// 简化的视线检查 - 检查玩家到目标位置的直线路径上是否有方块
	// 这是一个简化的实现，实际游戏中可能需要更复杂的算法
	dx := blockCenterX - playerCenterX
	dy := blockCenterY - playerCenterY
	steps := math.Max(math.Abs(dx), math.Abs(dy))
	
	if steps == 0 {
		return true
	}
	
	xStep := dx / steps
	yStep := dy / steps
	
	for i := 0.0; i < steps; i++ {
		x := playerCenterX + xStep*i
		y := playerCenterY + yStep*i
		
		// 检查当前位置是否与方块相交
		checkX := getBlockCoordinate(x)
		checkY := getBlockCoordinate(y)
		
		// 如果检查的位置不是目标位置且有方块，则视线被阻挡
		if checkX != blockX || checkY != blockY {
			if g.isBlockAt(checkX, checkY) {
				return false
			}
		}
	}
	
	return true
}

// isBlockAdjacent 检查指定位置是否与现有方块相邻（用于生存模式）
func (g *Game) isBlockAdjacent(x, y float64) bool {
	// 在生存模式下，必须与现有方块相邻才能放置
	// 检查四个方向是否有方块
	// 上
	if g.isBlockAt(x, y-BlockSize) {
		return true
	}
	// 下
	if g.isBlockAt(x, y+BlockSize) {
		return true
	}
	// 左
	if g.isBlockAt(x-BlockSize, y) {
		return true
	}
	// 右
	if g.isBlockAt(x+BlockSize, y) {
		return true
	}
	
	// 特殊情况：如果在地面层(y=0)放置方块，则认为是相邻的
	// 这允许玩家在地面上放置方块，而不需要跳跃
	if y == 0 {
		return true
	}
	
	return false
}

// distance 计算两点之间的距离
func distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// addBlock 在指定位置添加方块
func (g *Game) addBlock(x, y float64) {
	// 检查该位置是否已经有方块
	if !g.isBlockAt(x, y) {
		// 使用当前选中的物品类型
		blockType := g.currentItemType
		
		// 根据游戏模式应用不同的规则
		switch g.gameMode {
		case GameModeCreative:
			// 创造模式：可以隔着方块放置，无距离限制
			g.blocks = append(g.blocks, Block{x, y, BlockSize, BlockSize, blockType})
		case GameModeSurvival:
			// 生存模式：必须在距离范围内且与现有方块相邻
			playerCenterX := g.playerX + PlayerSize/2
			playerCenterY := g.playerY + PlayerSize/2
			blockCenterX := x + BlockSize/2
			blockCenterY := y + BlockSize/2
			
			// 计算玩家与方块之间的距离
			dist := distance(playerCenterX, playerCenterY, blockCenterX, blockCenterY)
			
			// 生存模式规则：
			// 1. 放置距离不能超过最大距离
			// 2. 必须与现有方块相邻
			if dist <= MaxPlaceDistance && g.isBlockAdjacent(x, y) {
				g.blocks = append(g.blocks, Block{x, y, BlockSize, BlockSize, blockType})
			}
		}
	}
}

// removeBlock 移除指定位置的方块
func (g *Game) removeBlock(x, y float64) {
	for i, block := range g.blocks {
		if block.X == x && block.Y == y {
			// 从切片中移除该方块并正确初始化新切片
			newBlocks := make([]Block, 0, len(g.blocks)-1)
			newBlocks = append(newBlocks, g.blocks[:i]...)
			newBlocks = append(newBlocks, g.blocks[i+1:]...)
			g.blocks = newBlocks
			break
		}
	}
}

// chunkKey 获取区块键值
func chunkKey(x, y int) string {
	return fmt.Sprintf("%d,%d", x, y)
}

// PerlinNoise 生成Perlin噪声值
type PerlinNoise struct {
	perm [512]int
}

// NewPerlinNoise 创建新的Perlin噪声生成器
func NewPerlinNoise(seed int64) *PerlinNoise {
	p := &PerlinNoise{}
	rand.Seed(seed)
	for i := range p.perm {
		p.perm[i] = i
	}
	rand.Shuffle(len(p.perm), func(i, j int) {
		p.perm[i], p.perm[j] = p.perm[j], p.perm[i]
	})
	return p
}

// Noise2D 生成2D Perlin噪声
func (p *PerlinNoise) Noise2D(x, y float64) float64 {
	X := int(math.Floor(x)) & 255
	Y := int(math.Floor(y)) & 255
	
	x -= math.Floor(x)
	y -= math.Floor(y)
	
	u := p.fade(x)
	v := p.fade(y)
	
	A := p.perm[X] + Y
	AA := p.perm[A & 511]
	AB := p.perm[(A+1) & 511]
	B := p.perm[(X+1) & 255] + Y
	BA := p.perm[B & 511]
	BB := p.perm[(B+1) & 511]
	
	return p.lerp(v, 
		p.lerp(u, p.grad(p.perm[AA & 511], x, y), 
			p.grad(p.perm[BA & 511], x-1, y)), 
		p.lerp(u, p.grad(p.perm[AB & 511], x, y-1), 
			p.grad(p.perm[BB & 511], x-1, y-1)))
}

// fade 淡化函数
func (p *PerlinNoise) fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

// lerp 线性插值
func (p *PerlinNoise) lerp(t, a, b float64) float64 {
	return a + t*(b-a)
}

// grad 梯度函数
func (p *PerlinNoise) grad(hash int, x, y float64) float64 {
	h := hash & 15
	u := x
	v := y
	if h < 8 {
		u = y
		v = x
	}
	if h&4 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	return u + v
}

// OctaveNoise 生成多层噪声（分形噪声）
func (p *PerlinNoise) OctaveNoise(octaves int, persistence, scale, x, y float64) float64 {
	var total float64
	var frequency, amplitude float64
	maxAmplitude := 0.0
	
	for i := 0; i < octaves; i++ {
		frequency = math.Pow(2, float64(i))
		amplitude = math.Pow(persistence, float64(i))
		
		total += p.Noise2D(x*scale*frequency, y*scale*frequency) * amplitude
		maxAmplitude += amplitude
	}
	
	return total / maxAmplitude
}

// TerrainGenerator 地形生成器
type TerrainGenerator struct {
	noise      *PerlinNoise
	seed       int64
}

// NewTerrainGenerator 创建新的地形生成器
func NewTerrainGenerator(seed int64) *TerrainGenerator {
	return &TerrainGenerator{
		noise: NewPerlinNoise(seed),
		seed:  seed,
	}
}

// getHeight 获取指定位置的高度
func (tg *TerrainGenerator) getHeight(x int) int {
	// 基础地形高度，调整垂直偏移使地面更接近玩家出生点
	baseHeight := tg.noise.OctaveNoise(4, 0.5, 0.01, float64(x), 0) * 20
	
	// 添加细节变化
	detail := tg.noise.OctaveNoise(3, 0.6, 0.05, float64(x), 100) * 5
	
	// 添加山脉
	mountains := 0.0
	if val := tg.noise.OctaveNoise(2, 0.7, 0.005, float64(x), 200); val > 0.6 {
		mountains = val * 20
	}
	
	// 调整整体高度偏移，使地面更适合玩家出生
	return int(baseHeight + detail + mountains) - 5
}

// getTerrainType 获取指定位置的地形类型
func (tg *TerrainGenerator) getTerrainType(x int) TerrainType {
	// 使用不同的噪声尺度获取地形类型
	continental := tg.noise.OctaveNoise(3, 0.5, 0.005, float64(x), 300)
	
	switch {
	case continental < -0.4:
		return TerrainTypeDesert
	case continental < -0.2:
		return TerrainTypeSavanna
	case continental < 0:
		return TerrainTypePlains
	case continental < 0.2:
		return TerrainTypeForest
	case continental < 0.4:
		return TerrainTypeHills
	case continental < 0.6:
		return TerrainTypeMountains
	default:
		return TerrainTypeSnowyPlains
	}
}

// getBlockType 获取指定位置和高度的方块类型
func (tg *TerrainGenerator) getBlockType(x, y, height int, terrainType TerrainType) ItemType {
	depth := height - y
	
	switch terrainType {
	case TerrainTypeDesert:
		if depth < 3 {
			return ItemTypeSand
		}
		return ItemTypeStone
		
	case TerrainTypeSavanna:
		if depth == 0 {
			return ItemTypeGrass
		} else if depth < 4 {
			return ItemTypeDirt
		}
		return ItemTypeStone
		
	case TerrainTypePlains:
		if depth == 0 {
			return ItemTypeGrass
		} else if depth < 3 {
			return ItemTypeDirt
		}
		return ItemTypeStone
		
	case TerrainTypeForest:
		if depth == 0 {
			return ItemTypeGrass
		} else if depth < 3 {
			return ItemTypeDirt
		}
		return ItemTypeStone
		
	case TerrainTypeHills:
		if depth == 0 {
			return ItemTypeGrass
		} else if depth < 5 {
			return ItemTypeDirt
		}
		return ItemTypeStone
		
	case TerrainTypeMountains:
		if depth == 0 {
			if y > 5 {
				return ItemTypeSnow
			}
			return ItemTypeStone
		} else if depth < 3 {
			return ItemTypeStone
		}
		return ItemTypeStone
		
	case TerrainTypeSnowyPlains:
		if depth == 0 {
			return ItemTypeSnow
		} else if depth < 3 {
			return ItemTypeDirt
		}
		return ItemTypeStone
		
	default:
		if depth == 0 {
			return ItemTypeGrass
		} else if depth < 3 {
			return ItemTypeDirt
		}
		return ItemTypeStone
	}
}


// hasCave 判断指定位置是否有洞穴
func (tg *TerrainGenerator) hasCave(x, y int) bool {
	// 使用噪声生成洞穴
	caveNoise := tg.noise.OctaveNoise(4, 0.6, 0.1, float64(x), float64(y)+1000)
	return caveNoise > 0.7 && y > -5
}

// hasTree 判断指定位置是否有树
func (tg *TerrainGenerator) hasTree(x, height int, terrainType TerrainType) bool {
	treeNoise := tg.noise.OctaveNoise(2, 0.5, 0.05, float64(x), 2000)
	
	switch terrainType {
	case TerrainTypeForest:
		return treeNoise > 0.6 && height >= 0
	case TerrainTypeJungle:
		return treeNoise > 0.5 && height >= 0
	case TerrainTypeTaiga:
		return treeNoise > 0.55 && height >= -2
	default:
		return false
	}
}

// getTreeHeight 获取树的高度
func (tg *TerrainGenerator) getTreeHeight(x int, terrainType TerrainType) int {
	treeNoise := tg.noise.OctaveNoise(2, 0.5, 0.1, float64(x), 3000)
	
	switch terrainType {
	case TerrainTypeForest:
		return 4 + int(treeNoise*4)
	case TerrainTypeJungle:
		return 6 + int(treeNoise*6)
	case TerrainTypeTaiga:
		return 5 + int(treeNoise*3)
	default:
		return 3 + int(treeNoise*3)
	}
}

// generateChunk 生成地形区块
func (g *Game) generateChunk(chunkX, chunkY int) *Chunk {
	chunk := &Chunk{
		X: chunkX,
		Y: chunkY,
	}
	
	// 初始化地形生成器
	terrainGen := NewTerrainGenerator(12345)
	
	// 确保在玩家出生点附近不会生成阻挡方块
	playerChunkX := int(math.Floor(g.playerX / ChunkWorldSize))
	isNearPlayerSpawn := (chunkX >= playerChunkX-1) && (chunkX <= playerChunkX+1) && chunkY == 0
	
	// 只在需要的区域内生成地形
	playerChunkY := int(math.Floor(g.playerY / ChunkWorldSize))
	if chunkY > playerChunkY+3 || chunkY < playerChunkY-3 {
		return chunk
	}
	
	// 为每个X坐标生成地形
	for x := 0; x < ChunkSize; x++ {
		worldX := chunkX*ChunkSize + x
		
		// 获取地形高度和类型
		height := terrainGen.getHeight(worldX)
		terrainType := terrainGen.getTerrainType(worldX)
		
		// 计算方块X坐标
		blockX := float64(worldX * BlockSize)
		
		// 在玩家出生点附近确保不会生成阻挡方块
		if isNearPlayerSpawn {
			playerSpawnY := -40.0
			spawnRadius := 3.0 * BlockSize
			if math.Abs(blockX) <= spawnRadius {
				blockY := float64(height * BlockSize)
				playerTop := playerSpawnY
				playerBottom := playerSpawnY + PlayerSize
				
				if blockY < playerBottom && (blockY + BlockSize) > playerTop {
					continue
				}
			}
		}
		
		// 生成地形柱
		maxDepth := 5
		switch terrainType {
		case TerrainTypeMountains:
			maxDepth = 8
		case TerrainTypeHills:
			maxDepth = 6
		case TerrainTypeDesert:
			maxDepth = 4
		}
		
		for y := height; y >= height-maxDepth; y-- {
			blockY := float64(y * BlockSize)
			
			// 检查是否在洞穴位置
			if terrainGen.hasCave(worldX, y) {
				continue
			}
			
			// 获取方块类型
			blockType := terrainGen.getBlockType(worldX, y, height, terrainType)
			
			// 添加方块到区块
			chunk.Blocks = append(chunk.Blocks, Block{
				X:    blockX,
				Y:    blockY,
				W:    BlockSize,
				H:    BlockSize,
				Type: blockType,
			})
		}
		
		// 生成树木
		if terrainGen.hasTree(worldX, height, terrainType) && !(isNearPlayerSpawn && math.Abs(blockX) <= 3*BlockSize) {
			treeHeight := terrainGen.getTreeHeight(worldX, terrainType)
			
			// 生成树干
			for i := 1; i <= treeHeight; i++ {
				chunk.Blocks = append(chunk.Blocks, Block{
					X:    blockX,
					Y:    float64(height+i) * BlockSize,
					W:    BlockSize,
					H:    BlockSize,
					Type: ItemTypeWood,
				})
			}
			
			// 生成树叶
			switch terrainType {
			case TerrainTypeForest, TerrainTypeJungle:
				// 简单的树冠
				chunk.Blocks = append(chunk.Blocks, Block{
					X:    blockX - BlockSize,
					Y:    float64(height+treeHeight+1) * BlockSize,
					W:    BlockSize * 3,
					H:    BlockSize,
					Type: ItemTypeGrass,
				})
				
				if terrainType == TerrainTypeJungle && treeHeight > 6 {
					chunk.Blocks = append(chunk.Blocks, Block{
						X:    blockX - BlockSize,
						Y:    float64(height+treeHeight-2) * BlockSize,
						W:    BlockSize * 3,
						H:    BlockSize,
						Type: ItemTypeGrass,
					})
				}
				
			case TerrainTypeTaiga:
				// 针叶树冠
				for i := 0; i < 3; i++ {
					chunk.Blocks = append(chunk.Blocks, Block{
						X:    blockX - float64(2-i)*BlockSize/2,
						Y:    float64(height+treeHeight-1+i) * BlockSize,
						W:    BlockSize * float64(3-i),
						H:    BlockSize,
						Type: ItemTypeGrass,
					})
				}
			}
		}
		
		// 在特定地形生成特殊元素
		switch terrainType {
		case TerrainTypeDesert:
			// 生成仙人掌
			cactusNoise := terrainGen.noise.OctaveNoise(2, 0.5, 0.1, float64(worldX), 4000)
			if cactusNoise > 0.7 && height >= 0 && !(isNearPlayerSpawn && math.Abs(blockX) <= 3*BlockSize) {
				cactusHeight := 1 + int(cactusNoise*3)
				for i := 1; i <= cactusHeight; i++ {
					chunk.Blocks = append(chunk.Blocks, Block{
						X:    blockX,
						Y:    float64(height+i) * BlockSize,
						W:    BlockSize,
						H:    BlockSize,
						Type: ItemTypeSand,
					})
				}
			}
			
		case TerrainTypeSwamp:
			// 生成水池
			waterNoise := terrainGen.noise.OctaveNoise(2, 0.5, 0.1, float64(worldX), 5000)
			if waterNoise > 0.6 && height >= -1 {
				chunk.Blocks = append(chunk.Blocks, Block{
					X:    blockX,
					Y:    float64(height) * BlockSize,
					W:    BlockSize,
					H:    BlockSize,
					Type: ItemTypeWater,
				})
			}
		}
	}
	
	return chunk
}

// loadChunk 加载区块（如果不存在则生成）
func (g *Game) loadChunk(chunkX, chunkY int) {
	key := chunkKey(chunkX, chunkY)
	if _, exists := g.chunks[key]; !exists {
		g.chunks[key] = g.generateChunk(chunkX, chunkY)
		g.blocks = append(g.blocks, g.chunks[key].Blocks...)
	}
}

// updateChunks 更新可见区块
func (g *Game) updateChunks() {
	// 计算玩家所在区块
	playerChunkX := int(math.Floor(g.playerX / ChunkWorldSize))
	playerChunkY := int(math.Floor(g.playerY / ChunkWorldSize))
	
	// 增加加载范围以提高性能和视觉效果
	visibleDistance := 3
	for x := playerChunkX - visibleDistance; x <= playerChunkX + visibleDistance; x++ {
		for y := playerChunkY - visibleDistance; y <= playerChunkY + visibleDistance; y++ {
			g.loadChunk(x, y)
		}
	}
	
	// 更新世界边界（无限世界不需要限制）
	// g.worldMinX = float64((playerChunkX - visibleDistance*2) * ChunkWorldSize)
	// g.worldMaxX = float64((playerChunkX + visibleDistance*2) * ChunkWorldSize)
	// g.worldMinY = float64((playerChunkY - visibleDistance*2) * ChunkWorldSize)
	// g.worldMaxY = float64((playerChunkY + visibleDistance*2) * ChunkWorldSize)
}

// getItemTypeAtHotbarPosition 获取物品栏中指定位置的物品类型
func (g *Game) getItemTypeAtHotbarPosition(pos int) ItemType {
	// 定义物品栏中的物品类型
	hotbarItems := []ItemType{
		ItemTypeGrass,
		ItemTypeDirt,
		ItemTypeStone,
		ItemTypeSand,
		ItemTypeWood,
		ItemTypeWater,
		ItemTypeLava,
		ItemTypeSnow,
	}
	
	// 确保索引在有效范围内
	if pos >= 0 && pos < len(hotbarItems) {
		return hotbarItems[pos]
	}
	
	// 默认返回草地
	return ItemTypeGrass
}

// getHotbarSize 获取物品栏大小
func (g *Game) getHotbarSize() int {
	return 8 // 8个物品槽位
}

// updateCurrentItemType 更新当前选中的物品类型
func (g *Game) updateCurrentItemType() {
	g.currentItemType = g.getItemTypeAtHotbarPosition(g.hotbarSelected)
}

// Update 处理游戏逻辑更新
func (g *Game) Update() error {
	// 初始化游戏
	if g.chunks == nil {
		g.chunks = make(map[string]*Chunk)
		// 初始化地形生成器
		terrainGen := NewTerrainGenerator(12345)
		// 获取出生点附近的地面高度
		spawnHeight := terrainGen.getHeight(0)
		// 初始化玩家位置 - 在地面略高的位置开始
		g.playerX = 0
		g.playerY = float64(spawnHeight * BlockSize - PlayerSize - 10) // 确保玩家出生时位于地面之上
		g.gameMode = GameModeCreative // 默认为创造模式
		g.hotbarSelected = 0          // 默认选择第一个物品
		g.updateCurrentItemType()
		
		// 确保玩家出生点周围没有方块
		// 清理玩家出生点附近的方块，确保玩家不会被卡住
		safeArea := 3.0 * BlockSize // 3个方块的半径
		var newBlocks []Block
		for _, block := range g.blocks {
			// 检查方块是否在玩家安全区域内
			// 使用方块坐标进行比较，而不是世界坐标
			blockCenterX := block.X + block.W/2
			blockCenterY := block.Y + block.H/2
			
			// 检查方块是否在玩家安全区域内（水平方向）
			if !(math.Abs(blockCenterX) <= safeArea &&
				blockCenterY >= g.playerY-BlockSize && blockCenterY <= g.playerY+PlayerSize+BlockSize) {
				newBlocks = append(newBlocks, block)
			}
		}
		g.blocks = newBlocks
	}
	
	// 切换游戏模式
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if g.gameMode == GameModeCreative {
			g.gameMode = GameModeSurvival
		} else {
			g.gameMode = GameModeCreative
		}
	}
	
	// 物品栏选择 (支持最多8个物品)
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.hotbarSelected = 0
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.hotbarSelected = 1
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		g.hotbarSelected = 2
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		g.hotbarSelected = 3
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		g.hotbarSelected = 4
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key6) {
		g.hotbarSelected = 5
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key7) {
		g.hotbarSelected = 6
		g.updateCurrentItemType()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key8) {
		g.hotbarSelected = 7
		g.updateCurrentItemType()
	}
	
	// 循环切换物品类型
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.hotbarSelected = (g.hotbarSelected + 1) % g.getHotbarSize()
		g.updateCurrentItemType()
	}
	
	// 鼠标滚轮切换物品
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		// 向上滚动，选择下一个物品
		g.hotbarSelected = (g.hotbarSelected + 1) % g.getHotbarSize()
		g.updateCurrentItemType()
	} else if wheelY < 0 {
		// 向下滚动，选择上一个物品
		g.hotbarSelected = (g.hotbarSelected + g.getHotbarSize() - 1) % g.getHotbarSize()
		g.updateCurrentItemType()
	}
	
	// 更新可见区块
	g.updateChunks()
	
	// 更新选中的方块（鼠标悬停的方块）
	mouseWorldX, mouseWorldY := g.getMouseWorldPosition()
	blockX := getBlockCoordinate(mouseWorldX)
	blockY := getBlockCoordinate(mouseWorldY)
	
	// 检查鼠标悬停位置是否有方块
	if g.isBlockAt(blockX, blockY) {
		g.selectedBlockX = blockX
		g.selectedBlockY = blockY
		g.hasSelectedBlock = true
	} else {
		g.hasSelectedBlock = false
	}
	
	// 处理框选
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		// 开始框选
		g.selecting = true
		g.selectionStartX, g.selectionStartY = g.getMouseWorldPosition()
		g.selectionEndX = g.selectionStartX
		g.selectionEndY = g.selectionStartY
	} else if g.selecting && ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		// 更新框选区域
		g.selectionEndX, g.selectionEndY = g.getMouseWorldPosition()
	} else if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonMiddle) {
		// 结束框选并放置方块
		if g.selecting {
			g.selecting = false
			// 确保起点坐标小于终点坐标
			minX := math.Min(g.selectionStartX, g.selectionEndX)
			maxX := math.Max(g.selectionStartX, g.selectionEndX)
			minY := math.Min(g.selectionStartY, g.selectionEndY)
			maxY := math.Max(g.selectionStartY, g.selectionEndY)
			
			// 在框选区域内放置方块
			for x := getBlockCoordinate(minX); x <= maxX; x += BlockSize {
				for y := getBlockCoordinate(minY); y <= maxY; y += BlockSize {
					// 检查视线（用于创造模式的远程放置）
					playerCenterX := g.playerX + PlayerSize/2
					playerCenterY := g.playerY + PlayerSize/2
					blockX := getBlockCoordinate(x)
					blockY := getBlockCoordinate(y)
					if g.hasLineOfSight(blockX, blockY, playerCenterX, playerCenterY) {
						g.addBlock(blockX, blockY)
					}
				}
			}
		}
	}
	
	// 处理方块破坏和放置
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// 左键破坏方块
		mouseWorldX, mouseWorldY := g.getMouseWorldPosition()
		blockX := getBlockCoordinate(mouseWorldX)
		blockY := getBlockCoordinate(mouseWorldY)
		g.removeBlock(blockX, blockY)
	} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		// 右键放置方块
		mouseWorldX, mouseWorldY := g.getMouseWorldPosition()
		blockX := getBlockCoordinate(mouseWorldX)
		blockY := getBlockCoordinate(mouseWorldY)
		
		// 检查视线（用于创造模式的远程放置）
		playerCenterX := g.playerX + PlayerSize/2
		playerCenterY := g.playerY + PlayerSize/2
		if g.hasLineOfSight(blockX, blockY, playerCenterX, playerCenterY) {
			g.addBlock(blockX, blockY)
		}
	}
	
	// 1. 处理玩家输入（水平移动）
	oldX := g.playerX
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		g.playerX -= PlayerSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		g.playerX += PlayerSpeed
	}
	
	// 1.5 检测水平碰撞
	playerRect := Block{g.playerX, g.playerY, PlayerSize, PlayerSize, 0} // 玩家视为类型0（无意义）
	for _, block := range g.blocks {
		if checkCollision(playerRect, block) {
			// 从左侧碰撞
			if oldX <= block.X - PlayerSize {
				g.playerX = block.X - PlayerSize
			// 从右侧碰撞
			} else if oldX >= block.X + block.W {
				g.playerX = block.X + block.W
			}
		}
	}
	
	// 边界检查（支持负数坐标）
	if g.worldMinX != 0 && g.worldMaxX != 0 { // 确保世界边界已初始化
		if g.playerX < g.worldMinX {
			g.playerX = g.worldMinX
		} else if g.playerX > g.worldMaxX - PlayerSize {
			g.playerX = g.worldMaxX - PlayerSize
		}
	}

	// 2. 处理跳跃
	if (ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyW)) && g.playerOnGround {
		g.playerVelocityY = -JumpPower
		g.playerOnGround = false
	}

	// 3. 应用重力
	g.playerVelocityY += Gravity
	if g.playerVelocityY > PlayerMaxFall {
		g.playerVelocityY = PlayerMaxFall
	}
	
	// 4. 更新玩家垂直位置
	oldY := g.playerY
	g.playerY += g.playerVelocityY
	
	// 5. 检测垂直碰撞
	g.playerOnGround = false
	playerRect = Block{g.playerX, g.playerY, PlayerSize, PlayerSize, 0} // 玩家视为类型0（无意义）
	
	for _, block := range g.blocks {
		if checkCollision(playerRect, block) {
			// 从上方落下碰撞
			if g.playerVelocityY > 0 && oldY <= block.Y - PlayerSize {
				g.playerY = block.Y - PlayerSize
				g.playerVelocityY = 0
				g.playerOnGround = true
			// 从下方撞击方块
			} else if g.playerVelocityY < 0 && oldY >= block.Y + block.H {
				g.playerY = block.Y + block.H
				g.playerVelocityY = 0
			}
		}
	}

	// 6. 计算摄像机目标位置（玩家中心位置）
	targetCameraX := -g.playerX + ScreenWidth/2 - PlayerSize/2
	targetCameraY := -g.playerY + ScreenHeight/2 - PlayerSize/2

	// 7. 平滑移动摄像机到目标位置
	g.cameraX += (targetCameraX - g.cameraX) * CameraLerp
	g.cameraY += (targetCameraY - g.cameraY) * CameraLerp

	return nil
}

// checkCollision 检测两个矩形是否碰撞
func checkCollision(a, b Block) bool {
	return a.X < b.X+b.W && 
		   a.X+a.W > b.X && 
		   a.Y < b.Y+b.H && 
		   a.Y+a.H > b.Y
}

// drawHotbar 绘制物品栏
func (g *Game) drawHotbar(screen *ebiten.Image) {
	const (
		hotbarX      = 10
		hotbarY      = ScreenHeight - 60
		slotSize     = 40
		slotSpacing  = 5
	)
	
	// 计算物品栏总宽度
	hotbarSize := g.getHotbarSize()
	hotbarWidth := hotbarSize*slotSize + (hotbarSize-1)*slotSpacing
	
	// 绘制物品栏背景
	ebitenutil.DrawRect(screen, float64(hotbarX-2), float64(hotbarY-2), 
		float64(hotbarWidth+4), float64(slotSize+4), 
		color.RGBA{0, 0, 0, 100})
	
	// 绘制每个物品栏槽位
	for i := 0; i < hotbarSize; i++ {
		x := hotbarX + i*(slotSize + slotSpacing)
		y := hotbarY
		
		// 绘制槽位背景
		slotColor := color.RGBA{100, 100, 100, 100}
		if i == g.hotbarSelected {
			slotColor = color.RGBA{200, 200, 200, 200} // 选中的槽位更亮
		}
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(slotSize), float64(slotSize), slotColor)
		
		// 绘制物品图标（简单矩形）
		itemType := g.getItemTypeAtHotbarPosition(i)
		item, exists := itemRegistry[itemType]
		if exists {
			itemColor := item.Color
			ebitenutil.DrawRect(screen, float64(x+5), float64(y+5), float64(slotSize-10), float64(slotSize-10), itemColor)
		}
		
		// 绘制数字键提示
		keyText := fmt.Sprintf("%d", i+1)
		ebitenutil.DebugPrintAt(screen, keyText, x+slotSize/2-4, y+slotSize+2)
	}
	
	// 绘制Q键提示
	ebitenutil.DebugPrintAt(screen, "Q: Cycle", hotbarX, hotbarY+slotSize+15)
	ebitenutil.DebugPrintAt(screen, "Wheel: Switch", hotbarX+80, hotbarY+slotSize+15)
}

// Draw 渲染游戏画面
func (g *Game) Draw(screen *ebiten.Image) {
	// 绘制背景
	screen.Fill(color.RGBA{30, 30, 60, 255})

	// 应用摄像头变换
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.cameraX, g.cameraY)

	// 绘制网格（帮助观察移动）
	gridSize := 50.0
	for x := math.Floor((g.cameraX - ScreenWidth/2) / gridSize) * gridSize; x <= math.Ceil((g.cameraX + ScreenWidth + ScreenWidth/2) / gridSize) * gridSize; x += gridSize {
		x0, y0 := op.GeoM.Apply(x, g.cameraY - ScreenHeight)
		x1, y1 := op.GeoM.Apply(x, g.cameraY + ScreenHeight*2)
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, color.Gray{100})
	}
	
	for y := math.Floor((g.cameraY - ScreenHeight/2) / gridSize) * gridSize; y <= math.Ceil((g.cameraY + ScreenHeight + ScreenHeight/2) / gridSize) * gridSize; y += gridSize {
		x0, y0 := op.GeoM.Apply(g.cameraX - ScreenWidth, y)
		x1, y1 := op.GeoM.Apply(g.cameraX + ScreenWidth*2, y)
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, color.Gray{100})
	}

	// 绘制地面方块
	for _, block := range g.blocks {
		x, y := op.GeoM.Apply(block.X, block.Y)
		// 根据方块类型改变颜色
		var blockColor color.RGBA
		item, exists := itemRegistry[block.Type]
		if exists {
			blockColor = item.Color
		} else {
			blockColor = color.RGBA{100, 200, 100, 255} // 默认绿色
		}
		ebitenutil.DrawRect(screen, x, y, block.W, block.H, blockColor)
	}

	// 绘制玩家（红色方块）
	x, y := op.GeoM.Apply(g.playerX, g.playerY)
	ebitenutil.DrawRect(screen, x, y, PlayerSize, PlayerSize, color.RGBA{255, 0, 0, 255})
	
	// 绘制选中方块的黑框
	if g.hasSelectedBlock {
		x, y := op.GeoM.Apply(g.selectedBlockX, g.selectedBlockY)
		// 绘制黑框（比方块稍大一点，确保可见）
		ebitenutil.DrawRect(screen, x-2, y-2, BlockSize+4, 2, color.RGBA{0, 0, 0, 255}) // 上边
		ebitenutil.DrawRect(screen, x-2, y+BlockSize, BlockSize+4, 2, color.RGBA{0, 0, 0, 255}) // 下边
		ebitenutil.DrawRect(screen, x-2, y, 2, BlockSize, color.RGBA{0, 0, 0, 255}) // 左边
		ebitenutil.DrawRect(screen, x+BlockSize, y, 2, BlockSize, color.RGBA{0, 0, 0, 255}) // 右边
	}
	
	// 绘制选择框
	if g.selecting {
		// 计算选择框的屏幕坐标
		startX, startY := op.GeoM.Apply(g.selectionStartX, g.selectionStartY)
		endX, endY := op.GeoM.Apply(g.selectionEndX, g.selectionEndY)
		
		// 确保绘制的矩形坐标正确（左上到右下）
		minX := math.Min(startX, endX)
		maxX := math.Max(startX, endX)
		minY := math.Min(startY, endY)
		maxY := math.Max(startY, endY)
		
		// 绘制选择框的四条边
		// 上边
		ebitenutil.DrawLine(screen, minX, minY, maxX, minY, color.RGBA{255, 255, 255, 255})
		// 下边
		ebitenutil.DrawLine(screen, minX, maxY, maxX, maxY, color.RGBA{255, 255, 255, 255})
		// 左边
		ebitenutil.DrawLine(screen, minX, minY, minX, maxY, color.RGBA{255, 255, 255, 255})
		// 右边
		ebitenutil.DrawLine(screen, maxX, minY, maxX, maxY, color.RGBA{255, 255, 255, 255})
	}

	// 调试信息
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Player: (%.1f, %.1f)", g.playerX, g.playerY), 10, 10)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Camera: (%.1f, %.1f)", g.cameraX, g.cameraY), 10, 30)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Velocity Y: %.2f", g.playerVelocityY), 10, 50)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("On Ground: %t", g.playerOnGround), 10, 70)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("World Bound: (%.0f,%.0f)-(%.0f,%.0f)", g.worldMinX, g.worldMinY, g.worldMaxX, g.worldMaxY), 10, 90)
	
	// 显示游戏模式
	modeText := "Mode: Creative"
	if g.gameMode == GameModeSurvival {
		modeText = "Mode: Survival"
	}
	ebitenutil.DebugPrintAt(screen, modeText, 10, 110)
	ebitenutil.DebugPrintAt(screen, "Press 'M' to switch mode", 10, 130)
	
	// 显示当前物品类型
	currentItem := itemRegistry[g.currentItemType]
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Item: %s", currentItem.Name), 10, 150)
	ebitenutil.DebugPrintAt(screen, "Press '1/2/3' or 'Q' to switch items", 10, 170)
	
	// 绘制物品栏
	g.drawHotbar(screen)
	
	// 显示框选提示
	if g.selecting {
		ebitenutil.DebugPrintAt(screen, "Selecting area...", 10, 190)
	} else {
		ebitenutil.DebugPrintAt(screen, "Middle mouse button to select area", 10, 190)
	}
}

// Layout 设置游戏窗口布局
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}


// main 程序入口点
func main() {
	// 我们使用自定义噪声函数生成地形，不需要随机种子
	
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("Smooth Camera Follow - Ebitengine")
	ebiten.SetWindowResizable(false)

	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}