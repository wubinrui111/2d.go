package main

import (
	"fmt"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)
//g.playerX = 0
const (
	ScreenWidth  = 640
	ScreenHeight = 480
	PlayerSize   = 50
	WorldWidth   = 2000  // 虚拟游戏世界宽度
	WorldHeight  = 2000  // 虚拟游戏世界高度
	PlayerSpeed  = 4.0   // 玩家移动速度
	CameraLerp   = 0.1   // 摄角机跟随速度 (0.01 ~ 0.3，越小越慢越平滑)
	
	// 物理常量
	Gravity       = 0.5
	JumpPower     = 12.0
	PlayerMaxFall = 10.0
	
	// 地形生成常量
	BlockSize     = 50
	ChunkSize     = 10              // 每个区块的方块数
	ChunkWorldSize = BlockSize * ChunkSize // 每个区块的世界尺寸
	GenerationDistance = 3          // 生成距离（以区块为单位）
	UndergroundDepth   = 10         // 地下深度
	
	// 游戏模式
	GameModeCreative = iota // 创造模式
	GameModeSurvival        // 生存模式
	
	// 生存模式参数
	MaxPlaceDistance = 5 * BlockSize
)

// 物品类型定义
type ItemType int

const (
	ItemTypeGrass ItemType = iota // 草地
	ItemTypeDirt                  // 泥土
	ItemTypeStone                 // 石头
)

// 物品定义结构
type Item struct {
	Type        ItemType
	Name        string
	Color       color.RGBA
	Description string
}

// 全局物品注册表
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
}

// 地面方块结构
type Block struct {
	X, Y, W, H float64
	Type       ItemType // 方块类型
}

// 区块结构
type Chunk struct {
	X, Y   int
	Blocks []Block
}

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

// 获取鼠标在世界中的位置
func (g *Game) getMouseWorldPosition() (float64, float64) {
	x, y := ebiten.CursorPosition()
	worldX := float64(x) - g.cameraX
	worldY := float64(y) - g.cameraY
	return worldX, worldY
}

// 获取方块坐标（将世界坐标转换为方块坐标）
func getBlockCoordinate(worldCoord float64) float64 {
	return math.Floor(worldCoord/BlockSize) * BlockSize
}

// 检查指定位置是否有方块（精确匹配方块坐标）
func (g *Game) isBlockAt(x, y float64) bool {
	for _, block := range g.blocks {
		// 精确比较方块的X和Y坐标，确保匹配指定位置
		if block.X == x && block.Y == y {
			return true
		}
	}
	return false
}

// 检查在指定位置和玩家之间是否有视线（用于创造模式）
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

// 检查指定位置是否与现有方块相邻（用于生存模式）
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

// 计算两点之间的距离
func distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// 在指定位置添加方块
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

// 移除指定位置的方块
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

// 获取区块键值
func chunkKey(x, y int) string {
	return fmt.Sprintf("%d,%d", x, y)
}

// 生成地形区块
func (g *Game) generateChunk(chunkX, chunkY int) *Chunk {
	chunk := &Chunk{
		X: chunkX,
		Y: chunkY,
	}
	
	// 只在y=0的位置生成无限延伸的水平地面
	if chunkY == 0 { // 只在y=0的区块生成地面
		for x := -100; x <= 100; x++ { // 生成足够长的地面
			blockX := x * BlockSize
			// 默认生成草地类型的方块
			chunk.Blocks = append(chunk.Blocks, Block{float64(blockX), 0, BlockSize, BlockSize, ItemTypeGrass})
		}
	}
	
	return chunk
}

// 加载区块（如果不存在则生成）
func (g *Game) loadChunk(chunkX, chunkY int) {
	key := chunkKey(chunkX, chunkY)
	if _, exists := g.chunks[key]; !exists {
		g.chunks[key] = g.generateChunk(chunkX, chunkY)
		g.blocks = append(g.blocks, g.chunks[key].Blocks...)
	}
}

// 更新可见区块
func (g *Game) updateChunks() {
	// 计算玩家所在区块
	playerChunkX := int(math.Floor(g.playerX / ChunkWorldSize))
	playerChunkY := int(math.Floor(g.playerY / ChunkWorldSize))
	
	// 加载玩家周围的区块
	for x := playerChunkX - GenerationDistance; x <= playerChunkX + GenerationDistance; x++ {
		for y := playerChunkY - GenerationDistance; y <= playerChunkY + GenerationDistance; y++ {
			g.loadChunk(x, y)
		}
	}
	
	// 更新世界边界
	g.worldMinX = float64((playerChunkX - GenerationDistance*2) * ChunkWorldSize)
	g.worldMaxX = float64((playerChunkX + GenerationDistance*2) * ChunkWorldSize)
	g.worldMinY = float64((playerChunkY - GenerationDistance*2) * ChunkWorldSize)
	g.worldMaxY = float64((playerChunkY + GenerationDistance*2) * ChunkWorldSize)
}

// 获取物品栏中指定位置的物品类型
func (g *Game) getItemTypeAtHotbarPosition(pos int) ItemType {
	switch pos {
	case 0:
		return ItemTypeGrass
	case 1:
		return ItemTypeDirt
	case 2:
		return ItemTypeStone
	default:
		return ItemTypeGrass
	}
}

// 更新当前选中的物品类型
func (g *Game) updateCurrentItemType() {
	g.currentItemType = g.getItemTypeAtHotbarPosition(g.hotbarSelected)
}

func (g *Game) Update() error {
	// 初始化游戏
	if g.chunks == nil {
		g.chunks = make(map[string]*Chunk)
		// 初始化玩家位置 - 在地面略高的位置开始
		g.playerX = 0
		g.playerY = -PlayerSize // 确保玩家生成在地面以上（地面在y=0，玩家高度为PlayerSize）
		g.gameMode = GameModeCreative // 默认为创造模式
		g.hotbarSelected = 0          // 默认选择第一个物品
		g.updateCurrentItemType()
	}
	
	// 切换游戏模式
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if g.gameMode == GameModeCreative {
			g.gameMode = GameModeSurvival
		} else {
			g.gameMode = GameModeCreative
		}
	}
	
	// 物品栏选择
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
	
	// 循环切换物品类型
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.hotbarSelected = (g.hotbarSelected + 1) % 3
		g.updateCurrentItemType()
	}
	
	// 鼠标滚轮切换物品
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		// 向上滚动，选择下一个物品
		g.hotbarSelected = (g.hotbarSelected + 1) % 3
		g.updateCurrentItemType()
	} else if wheelY < 0 {
		// 向下滚动，选择上一个物品
		g.hotbarSelected = (g.hotbarSelected + 2) % 3 // +2 等同于 -1 (mod 3)
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

// 检测两个矩形是否碰撞
func checkCollision(a, b Block) bool {
	return a.X < b.X+b.W && 
		   a.X+a.W > b.X && 
		   a.Y < b.Y+b.H && 
		   a.Y+a.H > b.Y
}

func (g *Game) drawHotbar(screen *ebiten.Image) {
	const (
		hotbarX      = 10
		hotbarY      = ScreenHeight - 60
		slotSize     = 40
		slotSpacing  = 5
	)
	
	// 绘制物品栏背景
	ebitenutil.DrawRect(screen, float64(hotbarX-2), float64(hotbarY-2), 
		float64(3*slotSize+2*slotSpacing+4), float64(slotSize+4), 
		color.RGBA{0, 0, 0, 100})
	
	// 绘制每个物品栏槽位
	for i := 0; i < 3; i++ {
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
}

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

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

func main() {
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("Smooth Camera Follow - Ebitengine")
	ebiten.SetWindowResizable(false)

	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}