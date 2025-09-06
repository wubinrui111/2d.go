package main

import (
	"fmt"
	"image/color"
	"log"

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
)

// 地面方块结构
type Block struct {
	X, Y, W, H float64
}

type Game struct {
	playerX, playerY   float64 // 玩家在世界中的位置
	playerVelocityY    float64 // 玩家垂直速度
	playerOnGround     bool    // 玩家是否在地面上

	// 实际摄像头偏移（用于绘制）
	cameraX, cameraY float64
	
	// 地面方块列表
	blocks []Block
}

func (g *Game) Update() error {
	// 初始化地面方块（网格位置相同的方块）
	if len(g.blocks) == 0 {
		// 在底部创建一排方块
		for x := 0; x < WorldWidth; x += 50 {
			g.blocks = append(g.blocks, Block{float64(x), WorldHeight - 50, 50, 50})
		}
		
		// 添加一些测试平台
		g.blocks = append(g.blocks, Block{200, WorldHeight - 200, 100, 50})
		g.blocks = append(g.blocks, Block{400, WorldHeight - 350, 100, 50})
		g.blocks = append(g.blocks, Block{600, WorldHeight - 500, 100, 50})
	}
	
	// 1. 处理玩家输入（水平移动）
	oldX := g.playerX
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.playerX -= PlayerSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
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
	
	// 边界检查
	if g.playerX < 0 {
		g.playerX = 0
	} else if g.playerX > WorldWidth-PlayerSize {
		g.playerX = WorldWidth - PlayerSize
	}

	// 2. 处理跳跃
	if ebiten.IsKeyPressed(ebiten.KeySpace) && g.playerOnGround {
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
	for x := 0; x <= WorldWidth; x += 50 {
		x0, y0 := op.GeoM.Apply(float64(x), 0)
		x1, y1 := op.GeoM.Apply(float64(x), WorldHeight)
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, color.Gray{100})
	}
	for y := 0; y <= WorldHeight; y += 50 {
		x0, y0 := op.GeoM.Apply(0, float64(y))
		x1, y1 := op.GeoM.Apply(WorldWidth, float64(y))
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, color.Gray{100})
	}

	// 绘制地面方块
	for _, block := range g.blocks {
		x, y := op.GeoM.Apply(block.X, block.Y)
		ebitenutil.DrawRect(screen, x, y, block.W, block.H, color.RGBA{100, 200, 100, 255})
	}

	// 绘制玩家（红色方块）
	x, y := op.GeoM.Apply(g.playerX, g.playerY)
	ebitenutil.DrawRect(screen, x, y, PlayerSize, PlayerSize, color.RGBA{255, 0, 0, 255})

	// 调试信息
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Player: (%.1f, %.1f)", g.playerX, g.playerY), 10, 10)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Camera: (%.1f, %.1f)", g.cameraX, g.cameraY), 10, 30)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Velocity Y: %.2f", g.playerVelocityY), 10, 50)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("On Ground: %t", g.playerOnGround), 10, 70)
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