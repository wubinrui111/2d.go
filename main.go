package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	PlayerSize   = 50
	WorldWidth   = 2000 // 虚拟游戏世界宽度
	WorldHeight  = 2000 // 虚拟游戏世界高度
	PlayerSpeed  = 4.0  // 玩家移动速度
	CameraLerp   = 0.1  // 摄像头跟随速度 (0.01 ~ 0.3，越小越慢越平滑)
	
	// 物理常量
	Gravity       = 0.5
	JumpPower     = 12.0
	PlayerMaxFall = 10.0
	
	// 地形生成常量
	BlockSize     = 50
	ChunkSize     = 10              // 每个区块的方块数
	ChunkWorldSize = BlockSize * ChunkSize // 每个区块的世界尺寸
	GenerationDistance = 3          // 生成距离（以区块为单位）
	
	// 地下世界生成参数
	UndergroundDepth = 5 // 地下层数
)

// 地面方块结构
type Block struct {
	X, Y, W, H float64
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
	
	// 使用确定性随机数生成地形
	r := rand.New(rand.NewSource(int64(chunkX*1000 + chunkY)))
	
	// 生成地下层和地面层
	for x := 0; x < ChunkSize; x++ {
		// 计算区块内的X坐标
		blockX := chunkX*ChunkSize + x
		
		// 生成不同深度的方块
		for y := -UndergroundDepth; y <= 5; y++ { // 从地下UndergroundDepth层到地上5层
			blockY := chunkY*ChunkSize + y
			
			// 有一定概率生成方块（地下层概率更高）
			probability := 0.3
			if y < 0 { // 地下层
				probability = 0.8
			} else if y == 0 { // 地面层
				probability = 1.0
			} else if y < 3 { // 地上几层
				probability = 0.2
			}
			
			if r.Float64() < probability {
				worldX := float64(blockX * BlockSize)
				worldY := float64(blockY * BlockSize)
				chunk.Blocks = append(chunk.Blocks, Block{worldX, worldY, BlockSize, BlockSize})
			}
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

func (g *Game) Update() error {
	// 初始化游戏
	if g.chunks == nil {
		g.chunks = make(map[string]*Chunk)
		// 初始化玩家位置 - 在地面层上方开始
		g.playerX = 0
		g.playerY = -100 // 在地面上方一些位置开始
	}
	
	// 更新可见区块
	g.updateChunks()
	
	// 1. 处理玩家输入（水平移动）
	oldX := g.playerX
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		g.playerX -= PlayerSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		g.playerX += PlayerSpeed
	}
	
	// 1.5 检测水平碰撞
	playerRect := Block{g.playerX, g.playerY, PlayerSize, PlayerSize}
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
	if g.playerX < g.worldMinX {
		g.playerX = g.worldMinX
	} else if g.playerX > g.worldMaxX - PlayerSize {
		g.playerX = g.worldMaxX - PlayerSize
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
	playerRect = Block{g.playerX, g.playerY, PlayerSize, PlayerSize}
	
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

	// 6. 如果玩家掉落到世界底部
	if g.playerY > WorldHeight-PlayerSize {
		g.playerY = WorldHeight - PlayerSize
		g.playerVelocityY = 0
		g.playerOnGround = true
	}
	
	if g.playerY < 0 {
		g.playerY = 0
	}

	// 7. 计算摄像机目标位置（玩家中心位置）
	targetCameraX := -g.playerX + ScreenWidth/2 - PlayerSize/2
	targetCameraY := -g.playerY + ScreenHeight/2 - PlayerSize/2

	// 8. 限制摄像头不越界
	clamp := func(x, min, max float64) float64 {
		if x < min {
			return min
		}
		if x > max {
			return max
		}
		return x
	}

	targetCameraX = clamp(targetCameraX, -(WorldWidth-ScreenWidth), 0)
	targetCameraY = clamp(targetCameraY, -(WorldHeight-ScreenHeight), 0)

	// 9. 平滑移动摄像机到目标位置
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
		// 根据方块的Y坐标改变颜色（地下更深的颜色）
		blockColor := color.RGBA{100, 200, 100, 255}
		if block.Y < 0 {
			// 地下使用不同颜色
			blockColor = color.RGBA{150, 100, 50, 255} // 土色
		} else if block.Y == 0 {
			// 地面层使用草色
			blockColor = color.RGBA{50, 180, 50, 255}
		}
		ebitenutil.DrawRect(screen, x, y, block.W, block.H, blockColor)
	}

	// 绘制玩家（红色方块）
	x, y := op.GeoM.Apply(g.playerX, g.playerY)
	ebitenutil.DrawRect(screen, x, y, PlayerSize, PlayerSize, color.RGBA{255, 0, 0, 255})

	// 调试信息
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Player: (%.1f, %.1f)", g.playerX, g.playerY), 10, 10)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Camera: (%.1f, %.1f)", g.cameraX, g.cameraY), 10, 30)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Velocity Y: %.2f", g.playerVelocityY), 10, 50)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("On Ground: %t", g.playerOnGround), 10, 70)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("World Bound: (%.0f,%.0f)-(%.0f,%.0f)", g.worldMinX, g.worldMinY, g.worldMaxX, g.worldMaxY), 10, 90)
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