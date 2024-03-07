// Copyright 2014 The go-gl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Renders a textured spinning cube using GLFW 3 and OpenGL 4.1 core forward-compatible profile.
package main // import "github.com/go-gl/example/gl41core-cube"

import (
	"fmt"
	"go/build"
	"image"
	"image/draw"
	_ "image/png"
	"log"
	"math/rand/v2"
	"os"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const windowWidth = 800
const windowHeight = 600

type Vector2DF struct {
	x float32
	y float32
}

func (vec Vector2DF) add(rhs Vector2DF) Vector2DF {
	return Vector2DF{vec.x + rhs.x, vec.y + rhs.y}
}

func (vec Vector2DF) subtract(rhs Vector2DF) Vector2DF {
	return Vector2DF{vec.x - rhs.x, vec.y - rhs.y}
}

func (vec Vector2DF) mul_scalar(scalar float32) Vector2DF {
	return Vector2DF{vec.x * scalar, vec.y * scalar}
}

type BoundingBox2D struct {
	top_left     Vector2DF
	bottom_right Vector2DF
}

func make_bounding_box_2d_vec(top_left Vector2DF, bottom_right Vector2DF) BoundingBox2D {
	return BoundingBox2D{top_left, bottom_right}
}

func make_bounding_box_2d_xy(x_min float32, x_max float32, y_min float32, y_max float32) BoundingBox2D {
	return BoundingBox2D{Vector2DF{x_min, y_max}, Vector2DF{x_max, y_min}}
}

type Player struct {
	pos   Vector2DF
	vel   Vector2DF
	accel Vector2DF

	vao uint32
	vbo uint32

	texture uint32
}

type Camera struct {
	pos2D Vector2DF

	z_value float32

	targetPos Vector2DF
}

type StaticMapEntity struct {
	pos     Vector2DF
	bb      BoundingBox2D
	texture uint32
}

func make_static_map_entity(pos Vector2DF, bb BoundingBox2D, texture_filename string) StaticMapEntity {
	texture, err := new_texture(texture_filename)

	if err != nil {
		log.Fatalf("Could not load texture %s", texture_filename)
		return StaticMapEntity{}
	}

	return StaticMapEntity{pos, bb, texture}
}

type Map struct {
	angle    float32
	entities []StaticMapEntity

	cube_vao uint32
	cube_vbo uint32
}

var g_Player = Player{}
var g_Camera = Camera{}
var g_Map = Map{}

func init_player(program uint32) {
	texture, err := new_texture("square.png")
	if err != nil {
		log.Fatalln(err)
		return
	}

	gl.GenVertexArrays(1, &g_Player.vao)
	gl.BindVertexArray(g_Player.vao)

	gl.GenBuffers(1, &g_Player.vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, g_Player.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(cubeVerticesPlayer)*4, gl.Ptr(cubeVerticesPlayer), gl.STATIC_DRAW)

	g_Player.texture = texture

	config_vertex_data(program)
}

func render_player(model_uniform_location int32) {
	model := mgl32.Translate3D(g_Player.pos.x, g_Player.pos.y, 0)

	gl.UniformMatrix4fv(model_uniform_location, 1, false, &model[0])

	gl.BindVertexArray(g_Player.vao)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, g_Player.texture)

	gl.DrawArrays(gl.TRIANGLES, 0, 6*2*3)
}

func render_map(model_uniform_location int32) {
	for _, entity := range g_Map.entities {
		model := mgl32.Translate3D(entity.pos.x, entity.pos.y, 0)

		gl.UniformMatrix4fv(model_uniform_location, 1, false, &model[0])

		gl.BindVertexArray(g_Map.cube_vao)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, entity.texture)

		gl.DrawArrays(gl.TRIANGLES, 0, 6*2*3)
	}
}

func step_player(dt float32) {
	g_Player.vel = g_Player.vel.add(g_Player.accel.mul_scalar(dt))
	g_Player.pos = g_Player.pos.add(g_Player.vel.mul_scalar(dt))

	g_Player.vel = g_Player.vel.mul_scalar(0.85)

	g_Player.accel = Vector2DF{0, 0}
}

func init_camera() {
	g_Camera.pos2D = Vector2DF{0.0, 0.0}
	g_Camera.z_value = 25.0
}

func step_camera(dt float32) {
	g_Camera.targetPos = g_Player.pos

	dt_scaled := min(dt*3, 1)

	diff_pos_target := g_Camera.targetPos.subtract(g_Camera.pos2D)

	g_Camera.pos2D = g_Camera.pos2D.add(diff_pos_target.mul_scalar(dt_scaled))
}

func update_camera_uniforms(cameraUniform int32) {
	cam_pos_3D := mgl32.Vec3{g_Camera.pos2D.x, g_Camera.pos2D.y, g_Camera.z_value}
	cam_look_at_pos := mgl32.Vec3{g_Camera.pos2D.x, g_Camera.pos2D.y, 0.0}
	up_direction := mgl32.Vec3{0, 1, 0}
	camera := mgl32.LookAtV(cam_pos_3D, cam_look_at_pos, up_direction)
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])
}

func init_map(program uint32) {
	gl.GenVertexArrays(1, &g_Map.cube_vao)
	gl.BindVertexArray(g_Map.cube_vao)

	fmt.Printf("g_Map.cube_vao %d\n", g_Map.cube_vao)

	gl.GenBuffers(1, &g_Map.cube_vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, g_Map.cube_vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(cubeVerticesMap)*4, gl.Ptr(cubeVerticesMap), gl.STATIC_DRAW)

	config_vertex_data(program)

	g_Map.angle = 0

	for i := -10; i < 10; i++ {
		block := make_static_map_entity(Vector2DF{float32(i) * rand.Float32(), float32(i) * rand.Float32()}, make_bounding_box_2d_xy(0, 2, 0, 2), "square.png")

		g_Map.entities = append(g_Map.entities, block)
	}

}

func step_map(dt float32) {
	g_Map.angle += dt
}

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

func config_vertex_data(program uint32) {
	// Configure the vertex data
	vertAttrib := uint32(gl.GetAttribLocation(program, gl.Str("vert\x00")))
	gl.EnableVertexAttribArray(vertAttrib)
	gl.VertexAttribPointerWithOffset(vertAttrib, 3, gl.FLOAT, false, 5*4, 0)

	texCoordAttrib := uint32(gl.GetAttribLocation(program, gl.Str("vertTexCoord\x00")))
	gl.EnableVertexAttribArray(texCoordAttrib)
	gl.VertexAttribPointerWithOffset(texCoordAttrib, 2, gl.FLOAT, false, 5*4, 3*4)
}

func main() {
	if err := glfw.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Game", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	// Initialize Glow
	if err := gl.Init(); err != nil {
		panic(err)
	}

	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version", version)

	// Configure the vertex and fragment shaders
	program, err := newProgram(vertexShader, fragmentShader)
	if err != nil {
		panic(err)
	}

	gl.UseProgram(program)

	projection := mgl32.Perspective(mgl32.DegToRad(45.0), float32(windowWidth)/windowHeight, 0.1, 1000.0)
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	init_camera()

	camera := mgl32.LookAtV(mgl32.Vec3{3, 3, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	model := mgl32.Ident4()
	modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])

	textureUniform := gl.GetUniformLocation(program, gl.Str("tex\x00"))
	gl.Uniform1i(textureUniform, 0)

	gl.BindFragDataLocation(program, 0, gl.Str("outputColor\x00"))

	init_player(program)
	init_map(program)

	// Configure global settings
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(1.0, 1.0, 1.0, 1.0)

	angle := 0.0
	previousTime := glfw.GetTime()

	for !window.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// Update
		time := glfw.GetTime()
		elapsed := time - previousTime
		previousTime = time

		elapsed_float32 := float32(elapsed)

		// Render
		gl.UseProgram(program)

		update_camera_uniforms(cameraUniform)
		render_map(modelUniform)
		render_player(modelUniform)

		// Maintenance
		window.SwapBuffers()

		// Controls
		add_accel := float32(100.0)
		if window.GetKey(glfw.KeyUp) == glfw.Press {
			angle += 0.5
			g_Player.accel = g_Player.accel.add(Vector2DF{0.0, +add_accel})
		}
		if window.GetKey(glfw.KeyDown) == glfw.Press {
			g_Player.accel = g_Player.accel.add(Vector2DF{0.0, -add_accel})
		}
		if window.GetKey(glfw.KeyLeft) == glfw.Press {
			g_Player.accel = g_Player.accel.add(Vector2DF{-add_accel, 0.0})
		}
		if window.GetKey(glfw.KeyRight) == glfw.Press {
			g_Player.accel = g_Player.accel.add(Vector2DF{add_accel, 0.0})
		}

		glfw.PollEvents()

		// Physics/Game steping
		step_player(elapsed_float32)
		step_camera(elapsed_float32)
		step_map(elapsed_float32)
	}
}

func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

func new_texture(file string) (uint32, error) {
	imgFile, err := os.Open(file)
	if err != nil {
		return 0, fmt.Errorf("texture %q not found on disk: %v", file, err)
	}
	img, _, err := image.Decode(imgFile)
	if err != nil {
		return 0, err
	}

	rgba := image.NewRGBA(img.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return 0, fmt.Errorf("unsupported stride")
	}
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	texture := uint32(0)
	gl.GenTextures(1, &texture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))

	return texture, nil
}

var vertexShader = `
#version 330

uniform mat4 projection;
uniform mat4 camera;
uniform mat4 model;

in vec3 vert;
in vec2 vertTexCoord;

out vec2 fragTexCoord;

void main() {
    fragTexCoord = vertTexCoord;
    gl_Position = projection * camera * model * vec4(vert, 1);
}
` + "\x00"

var fragmentShader = `
#version 330

uniform sampler2D tex;

in vec2 fragTexCoord;

out vec4 outputColor;

void main() {
    outputColor = texture(tex, fragTexCoord);
}
` + "\x00"

var cubeVerticesPlayer = []float32{
	//  X, Y, Z, U, V
	// Bottom
	-1.0, -1.0, -1.0, 0.0, 0.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	-1.0, -1.0, 1.0, 0.0, 1.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	1.0, -1.0, 1.0, 1.0, 1.0,
	-1.0, -1.0, 1.0, 0.0, 1.0,

	// Top
	-1.0, 1.0, -1.0, 0.0, 0.0,
	-1.0, 1.0, 1.0, 0.0, 1.0,
	1.0, 1.0, -1.0, 1.0, 0.0,
	1.0, 1.0, -1.0, 1.0, 0.0,
	-1.0, 1.0, 1.0, 0.0, 1.0,
	1.0, 1.0, 1.0, 1.0, 1.0,

	// Front
	-1.0, -1.0, 1.0, 1.0, 0.0,
	1.0, -1.0, 1.0, 0.0, 0.0,
	-1.0, 1.0, 1.0, 1.0, 1.0,
	1.0, -1.0, 1.0, 0.0, 0.0,
	1.0, 1.0, 1.0, 0.0, 1.0,
	-1.0, 1.0, 1.0, 1.0, 1.0,

	// Back
	-1.0, -1.0, -1.0, 0.0, 0.0,
	-1.0, 1.0, -1.0, 0.0, 1.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	-1.0, 1.0, -1.0, 0.0, 1.0,
	1.0, 1.0, -1.0, 1.0, 1.0,

	// Left
	-1.0, -1.0, 1.0, 0.0, 1.0,
	-1.0, 1.0, -1.0, 1.0, 0.0,
	-1.0, -1.0, -1.0, 0.0, 0.0,
	-1.0, -1.0, 1.0, 0.0, 1.0,
	-1.0, 1.0, 1.0, 1.0, 1.0,
	-1.0, 1.0, -1.0, 1.0, 0.0,

	// Right
	1.0, -1.0, 1.0, 1.0, 1.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	1.0, 1.0, -1.0, 0.0, 0.0,
	1.0, -1.0, 1.0, 1.0, 1.0,
	1.0, 1.0, -1.0, 0.0, 0.0,
	1.0, 1.0, 1.0, 0.0, 1.0,
}

var cubeVerticesMap = []float32{
	//  X, Y, Z, U, V
	// Bottom
	-1.0, -1.0, -1.0, 0.0, 0.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	-1.0, -1.0, 1.0, 0.0, 1.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	1.0, -1.0, 1.0, 1.0, 1.0,
	-1.0, -1.0, 1.0, 0.0, 1.0,

	// Top
	-1.0, 1.0, -1.0, 0.0, 0.0,
	-1.0, 1.0, 1.0, 0.0, 1.0,
	1.0, 1.0, -1.0, 1.0, 0.0,
	1.0, 1.0, -1.0, 1.0, 0.0,
	-1.0, 1.0, 1.0, 0.0, 1.0,
	1.0, 1.0, 1.0, 1.0, 1.0,

	// Front
	-1.0, -1.0, 1.0, 1.0, 0.0,
	1.0, -1.0, 1.0, 0.0, 0.0,
	-1.0, 1.0, 1.0, 1.0, 1.0,
	1.0, -1.0, 1.0, 0.0, 0.0,
	1.0, 1.0, 1.0, 0.0, 1.0,
	-1.0, 1.0, 1.0, 1.0, 1.0,

	// Back
	-1.0, -1.0, -1.0, 0.0, 0.0,
	-1.0, 1.0, -1.0, 0.0, 1.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	-1.0, 1.0, -1.0, 0.0, 1.0,
	1.0, 1.0, -1.0, 1.0, 1.0,

	// Left
	-1.0, -1.0, 1.0, 0.0, 1.0,
	-1.0, 1.0, -1.0, 1.0, 0.0,
	-1.0, -1.0, -1.0, 0.0, 0.0,
	-1.0, -1.0, 1.0, 0.0, 1.0,
	-1.0, 1.0, 1.0, 1.0, 1.0,
	-1.0, 1.0, -1.0, 1.0, 0.0,

	// Right
	1.0, -1.0, 1.0, 1.0, 1.0,
	1.0, -1.0, -1.0, 1.0, 0.0,
	1.0, 1.0, -1.0, 0.0, 0.0,
	1.0, -1.0, 1.0, 1.0, 1.0,
	1.0, 1.0, -1.0, 0.0, 0.0,
	1.0, 1.0, 1.0, 0.0, 1.0,
}

// Set the working directory to the root of Go package, so that its assets can be accessed.
func init() {
	dir, err := importPathToDir("github.com/go-gl/example/gl41core-cube")
	if err != nil {
		log.Fatalln("Unable to find Go package in your GOPATH, it's needed to load assets:", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		log.Panicln("os.Chdir:", err)
	}
}

// importPathToDir resolves the absolute path from importPath.
// There doesn't need to be a valid Go package inside that import path,
// but the directory must exist.
func importPathToDir(importPath string) (string, error) {
	p, err := build.Import(importPath, "", build.FindOnly)
	if err != nil {
		return "", err
	}
	return p.Dir, nil
}
