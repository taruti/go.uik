package layouts

import (
	"code.google.com/p/draw2d/draw2d"
	"github.com/skelterjohn/geom"
	"github.com/skelterjohn/go.uik"
	"image/color"
	"image/draw"
	"math"
)

func VBox(config GridConfig, blocks ...*uik.Block) (g *Grid) {
	g = NewGrid(config)
	for i, b := range blocks {
		g.Add <- BlockData{
			Block: b,
			GridX: 0, GridY: i,
			AnchorX: AnchorMin,
		}
	}
	return
}

func HBox(config GridConfig, blocks ...*uik.Block) (g *Grid) {
	g = NewGrid(config)
	for i, b := range blocks {
		g.Add <- BlockData{
			Block: b,
			GridX: i, GridY: 0,
			AnchorY: AnchorMin,
		}
	}
	return
}

type Anchor uint8

const (
	AnchorMin Anchor = 1 << iota
	AnchorMax
)

type BlockData struct {
	Block            *uik.Block
	GridX, GridY     int
	ExtraX, ExtraY   int
	AnchorX, AnchorY Anchor
}

type GridConfig struct {
}

type Grid struct {
	uik.Foundation

	children          map[*uik.Block]bool
	childrenBlockData map[*uik.Block]BlockData
	config            GridConfig

	vflex, hflex *flex

	Add       chan<- BlockData
	add       chan BlockData
	Remove    chan<- *uik.Block
	remove    chan *uik.Block
	SetConfig chan<- GridConfig
	setConfig chan GridConfig
	GetConfig <-chan GridConfig
	getConfig chan GridConfig
}

func NewGrid(cfg GridConfig) (g *Grid) {
	g = new(Grid)

	g.config = cfg

	g.Initialize()
	if uik.ReportIDs {
		uik.Report(g.ID, "grid")
	}

	go g.handleEvents()

	return
}

func (g *Grid) Initialize() {
	g.Foundation.Initialize()
	g.DrawOp = draw.Over

	g.children = map[*uik.Block]bool{}
	g.childrenBlockData = map[*uik.Block]BlockData{}

	g.add = make(chan BlockData, 1)
	g.Add = g.add
	g.remove = make(chan *uik.Block, 1)
	g.Remove = g.remove
	g.setConfig = make(chan GridConfig, 1)
	g.SetConfig = g.setConfig
	g.getConfig = make(chan GridConfig, 1)
	g.GetConfig = g.getConfig

	g.Paint = func(gc draw2d.GraphicContext) {
		g.draw(gc)
	}
}

func (g *Grid) addBlock(bd BlockData) {
	g.AddBlock(bd.Block)
	g.children[bd.Block] = true
	g.childrenBlockData[bd.Block] = bd
	g.vflex = nil
	g.regrid()
}

func (g *Grid) remBlock(b *uik.Block) {
	if !g.children[b] {
		return
	}
	delete(g.ChildrenHints, b)
	delete(g.childrenBlockData, b)
	g.vflex = nil
	g.regrid()
}

func (g *Grid) reflex() {
	if g.vflex != nil {
		return
	}
	g.hflex = &flex{}
	g.vflex = &flex{}
	for _, bd := range g.childrenBlockData {
		csh := g.ChildrenHints[bd.Block]
		g.hflex.add(&elem{
			index:    bd.GridX,
			extra:    bd.ExtraX,
			minSize:  csh.MinSize.X,
			prefSize: csh.PreferredSize.X,
			maxSize:  math.Inf(1),
		})
		g.vflex.add(&elem{
			index:    bd.GridY,
			extra:    bd.ExtraY,
			minSize:  csh.MinSize.Y,
			prefSize: csh.PreferredSize.Y,
			maxSize:  math.Inf(1),
		})
	}
}

func (g *Grid) makePreferences() {
	var sizeHint uik.SizeHint
	g.reflex()
	hmin, hpref, hmax := g.hflex.makePrefs()
	vmin, vpref, vmax := g.vflex.makePrefs()
	sizeHint.MinSize = geom.Coord{hmin, vmin}
	sizeHint.PreferredSize = geom.Coord{hpref, vpref}
	sizeHint.MaxSize = geom.Coord{hmax, vmax}
	g.SetSizeHint(sizeHint)
}

func (g *Grid) regrid() {
	g.reflex()

	_, minXs, maxXs := g.hflex.constrain(g.Size.X)
	_, minYs, maxYs := g.vflex.constrain(g.Size.Y)

	for child, csh := range g.ChildrenHints {
		bd := g.childrenBlockData[child]
		gridBounds := geom.Rect{
			Min: geom.Coord{
				X: minXs[bd.GridX],
				Y: minYs[bd.GridY],
			},
			Max: geom.Coord{
				X: maxXs[bd.GridX+bd.ExtraX],
				Y: maxYs[bd.GridY+bd.ExtraY],
			},
		}

		gridSizeX, gridSizeY := gridBounds.Size()
		if gridSizeX > csh.MaxSize.X {
			diff := gridSizeX - csh.MaxSize.X
			if bd.AnchorX&AnchorMin != 0 && bd.AnchorX&AnchorMax != 0 {
				gridBounds.Min.X += diff / 2
				gridBounds.Max.X -= diff / 2
			} else if bd.AnchorX&AnchorMin != 0 {
				gridBounds.Max.X -= diff
			} else if bd.AnchorX&AnchorMax != 0 {
				gridBounds.Min.X += diff
			}
		}
		if gridSizeY > csh.MaxSize.Y {
			diff := gridSizeY - csh.MaxSize.Y
			if bd.AnchorY&AnchorMin == 0 && bd.AnchorY&AnchorMax == 0 {
				gridBounds.Min.Y += diff / 2
				gridBounds.Max.Y -= diff / 2
			} else if bd.AnchorY&AnchorMin != 0 {
				gridBounds.Max.Y -= diff
			} else if bd.AnchorY&AnchorMax != 0 {
				gridBounds.Min.Y += diff
			}
		}

		gridSizeX, gridSizeY = gridBounds.Size()
		if gridSizeX > csh.PreferredSize.X {
			diff := gridSizeX - csh.PreferredSize.X
			if bd.AnchorX&AnchorMin != 0 && bd.AnchorX&AnchorMax != 0 {
				gridBounds.Min.X += diff / 2
				gridBounds.Max.X -= diff / 2
			} else if bd.AnchorX&AnchorMin != 0 {
				gridBounds.Max.X -= diff
			} else if bd.AnchorX&AnchorMax != 0 {
				gridBounds.Min.X += diff
			}
		}
		if gridSizeY > csh.PreferredSize.Y {
			diff := gridSizeY - csh.PreferredSize.Y
			if bd.AnchorY&AnchorMin == 0 && bd.AnchorY&AnchorMax == 0 {
				gridBounds.Min.Y += diff / 2
				gridBounds.Max.Y -= diff / 2
			} else if bd.AnchorY&AnchorMin != 0 {
				gridBounds.Max.Y -= diff
			} else if bd.AnchorY&AnchorMax != 0 {
				gridBounds.Min.Y += diff
			}
		}

		g.ChildrenBounds[child] = gridBounds

		gridSizeX, gridSizeY = gridBounds.Size()
		child.UserEventsIn <- uik.ResizeEvent{
			Size: geom.Coord{gridSizeX, gridSizeY},
		}
	}

	g.Invalidate()
}

func safeRect(path draw2d.GraphicContext, min, max geom.Coord) {
	x1, y1 := min.X, min.Y
	x2, y2 := max.X, max.Y
	x, y := path.LastPoint()
	path.MoveTo(x1, y1)
	path.LineTo(x2, y1)
	path.LineTo(x2, y2)
	path.LineTo(x1, y2)
	path.Close()
	path.MoveTo(x, y)
}

func (g *Grid) draw(gc draw2d.GraphicContext) {
	gc.Clear()
	gc.SetFillColor(color.RGBA{150, 150, 150, 255})
	safeRect(gc, geom.Coord{0, 0}, g.Size)
	gc.FillStroke()
}

func (g *Grid) handleEvents() {
	for {
		select {
		case e := <-g.UserEvents:
			switch e := e.(type) {
			case uik.ResizeEvent:
				g.Size = e.Size
				g.regrid()
				g.Invalidate()
			default:
				g.Foundation.HandleEvent(e)
			}
		case bsh := <-g.BlockSizeHints:
			if !g.children[bsh.Block] {
				// Do I know you?
				break
			}
			g.ChildrenHints[bsh.Block] = bsh.SizeHint

			g.vflex = nil

			g.makePreferences()
			g.regrid()
		case e := <-g.BlockInvalidations:
			g.DoBlockInvalidation(e)
			// go uik.ShowBuffer("grid", g.Buffer)
		case g.config = <-g.setConfig:
			g.vflex = nil
			g.makePreferences()
		case g.getConfig <- g.config:
		case bd := <-g.add:
			g.addBlock(bd)
			g.vflex = nil
			g.makePreferences()
		case b := <-g.remove:
			g.remBlock(b)
			g.vflex = nil
			g.makePreferences()
		}
	}
}
