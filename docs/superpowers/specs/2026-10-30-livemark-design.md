# livemark 设计：公开语法树 + CSS 主题

- **状态**：已批准（2026-10-30），实施中
- **日期**：2026-10-30
- **仓库**：`github.com/CarlOpenLab/livemark`（本地 `~/Desktop/carl-github/livemark`）
- **基线**：`fd86c85`——从 Muse `0c7dda1` 加当时工作区的改动 fork 出来的编辑器，构建与测试通过
- **已定的方向**：最终用户可写的主题文件；固定语法 + 公开语法树；主题是语法树上的真层叠（CSS 子集）；独立项目，Muse 暂不迁移

## 1. 背景与目标

Muse 的所见即所得编辑器已经原样 fork 到这里。它的外观写死在三处：一个私有的 `look` 结构（十几个颜色，深浅色两个分支）、`layout.go` 里的间距常量、`paint.go` 里的几处颜色。它对文档结构的理解（块、容器、行内样式）也是私有的，而且和排版、绘制写在一起。

v1 的目标：

1. **公开语法树。** 新增 `syntax` 包，纯 Go，不依赖 UI。它给出编辑器眼中的文档结构：每个节点的类型、源码区间、语法符号区间。应用可以拿它做大纲、导出、统计；主题把节点类型当作选择器的词汇。
2. **最终用户能写的 CSS 主题。** 主题是一个 CSS 子集文件，选择器作用在语法树上，经过层叠和继承得到每个节点的计算样式。编辑器只读计算样式，不再有任何写死的外观——默认外观本身就是一个 `default.css`。
3. **默认外观和 fork 时完全一致。** 像素级对照测试保证重构不改变样子。
4. **Playground。** 没有 Muse 当第一个使用者，所以要有一个 playground 来验证组件，也给主题作者用：边改 CSS 边看效果、报错面板、节点路径检查器。它像 MyGo 的文档预览一样，桌面能跑，也能编译成 WebAssembly 放在网页上。

v1 不做：

- 自定义语法插件。节点类型的命名空间给它留了位置，v2 再做。
- 完整的 CSS。没有布局类属性（`display`、`flex`、`position`、`float`），没有 `::before`/`::after` 生成内容，没有 `:hover`，没有 `@import`。
- Muse 迁移到 livemark。两边各自演进，以后再说。
- 代码文件模式（今天的 `Options.Code`）。这件事交给 MyGo 的 `ui/editor`。

## 2. 包结构

```
livemark/
├── syntax/          公开语法树：纯 Go，只依赖 MyGo 的 parser 包
├── theme/           CSS 子集：解析、选择器、层叠、继承、计算样式、诊断；纯 Go，不依赖 MyGo
│   └── default.css  内置默认主题（go:embed）
├── *.go             根包 livemark：MyGo 组件 Editor
├── render/math/     可选：go-latex 公式
├── render/mermaid/  可选：Mermaid 子集
├── internal/parse/  解析准备与 MyGo 文档，syntax 和编辑器共用
└── cmd/playground/  playground：桌面应用，也编译成 WebAssembly 放在网页上
```

依赖方向：

- `syntax` → `theme`、`internal/parse` → MyGo 的 `ui/markdown/parser`（只依赖标准库）
- `theme` → 标准库
- 根包 → `syntax`、`theme`、`internal/parse`、MyGo `ui`
- `render/*` → 根包定义的接口，加各自的依赖（go-latex 只在 `render/math` 里）
- `cmd/playground` → 全部

`Element`（选择器匹配的对象）由 `theme` 定义，`syntax` 用它描述节点。两个包都是纯 Go，所以 `syntax` 依赖 `theme` 不会多出任何外部依赖，也避免了两个同名类型。

`theme` 刻意不依赖 MyGo：颜色用它自己的 `theme.Color`，窗口主题的颜色由编辑器以变量（`--ui-*`）的形式传进来，可用字体也由编辑器去查。这样没有窗口的工具（比如 CI 里校验主题文件）也能解析主题。

## 3. 语法层 `syntax`

### 3.1 职责

- **解析准备。** 今天散在编辑器里的三件事搬到 `internal/parse`：front matter 遮罩、中文环境下的强调修正（`cjkMask`）、扩展集合（GFM、脚注、公式、`$…$`）。这样单独调用 `syntax.Parse` 得到的结构，和编辑器里看到的一致。
- **公开的只读语法树。** 包住 MyGo 的 `parser.Document`，但 API 上不出现 MyGo 的类型。MyGo 还在 0.x，它的 API 变了，livemark 的公开 API 不必跟着变。
- **元素映射。** 每个节点给出它在主题眼中的样子：`Element()`，即类型、类、属性。编辑器构建样式路径时用的是同一套映射，所以"检查器里看到的路径"就是"主题匹配的路径"。

### 3.2 API 草图

```go
package syntax

// Parse reads Markdown as the editor reads it.
func Parse(src string) *Tree

type Tree struct{ /* … */ }

func (t *Tree) Source() string
func (t *Tree) Root() Node
func (t *Tree) Walk() iter.Seq[Node]  // document order, depth first
func (t *Tree) PathAt(off int) []Node // from the root to the innermost node at a byte offset

// Node is a node of a tree, a value.
type Node struct{ /* tree, index */ }

func (n Node) Kind() Kind             // Paragraph, Heading, Strong, Syntax, …
func (n Node) Element() theme.Element // what selectors match
func (n Node) Range() Range           // its bytes in the source
func (n Node) Syntax() []Range        // the bytes of its syntax: **, `, [, ](url), #, >, fences
func (n Node) Parent() (Node, bool)
func (n Node) Children() iter.Seq[Node]
// By kind: Level, Lang, Info, Dest, Title, Task, Ordered, Start, Align, Label, Literal.
```

`theme.Element` 包括类型（`"heading"`）、类（`["h2"]`）、属性（`level="2"`）和状态（`:revealed` 等）。

- 一棵 `Tree` 是某一时刻源码的快照，只读。
- 行内节点按块懒构建，以 MyGo 的 `Block.Revision()` 为键缓存，没改动的块不重建。
- `ed.Syntax()` 返回编辑器当前源码的快照，按源码版本缓存。它不和编辑器内部的增量解析共享可变状态，所以拿到手的树不会在下一次编辑后悄悄变掉。
- 编辑器内部继续直接用 `internal/parse`（今天 9 个文件直接遍历 MyGo 的块和行内节点），不改成遍历 `syntax.Tree`。两边的一致性由 §7 的一致性测试保证。

### 3.3 元素词汇表

**这张表是写给最终用户的公开约定。**改名就会让用户的主题失效，所以 v1 要定稳。

命名规则：类型是小写名词；变体用类（`.h2`、`.inline`）；数据用属性（`[lang="go"]`）。

**块元素**

| 元素 | 类 | 属性 | 说明 |
|---|---|---|---|
| `document` | | | 根节点，`:root` 匹配它 |
| `paragraph` | | | |
| `heading` | `.h1`…`.h6`、`.setext` | `level` | |
| `quote` | | | 引用；容器 |
| `list` | `.ordered`/`.bullet`、`.tight`/`.loose` | `start`、`depth` | 容器；`depth` 从 0 起 |
| `item` | `.task`、`.checked` | `number`、`depth` | 容器 |
| `code` | `.block`、`.fenced`/`.indented` | `lang`、`info` | 代码块 |
| `math` | `.block` | | 块公式 |
| `html` | `.block` | | HTML 块 |
| `table` | | | |
| `row` | `.header`、`.odd`/`.even` | | 奇偶从表头之后的第一行数起 |
| `cell` | `.header` | `align` | `align` 取自 Markdown，没有对齐时不出现 |
| `rule` | | | 分隔线 |
| `footnote` | | `label` | 脚注定义；容器 |
| `front-matter` | | | YAML front matter |
| `link-def` | | | 链接引用定义 |

**行内元素**

| 元素 | 类 | 属性 | 来源 |
|---|---|---|---|
| `strong` | `.html` | | `**x**`、`<b>`、`<strong>` |
| `emphasis` | `.html` | | `*x*`、`<i>`、`<em>` |
| `strike` | `.html` | | `~~x~~`、`<s>`、`<del>`、`<strike>` |
| `underline` | | | `<u>`、`<ins>` |
| `mark` | | | `<mark>` |
| `sup`、`sub` | | | `<sup>`、`<sub>` |
| `kbd` | | | `<kbd>` |
| `code` | `.inline`、`.html` | | `` `x` ``、`<code>` |
| `link` | `.auto` | `href`、`title` | `.auto`：直接写网址的链接 |
| `image` | `.alone` | `src`、`alt`、`title` | `.alone`：段落里只有这一张图 |
| `math` | `.inline`、`.display` | | `$x$`；`.display` 是行内的 `$$x$$` |
| `footnote-ref` | | `label` | `[^1]` |
| `html` | `.inline` | `tag` | 不是格式标签的 HTML，原样显示 |
| `syntax` | 所属构造的名字 | | 光标处显示的语法符号，见下 |
| `token` | 词法单元的种类 | | 代码块里的高亮单元 |

`syntax` 是光标停在某个构造上时显示出来的语法符号。它有两条规则：

- **类是它所属构造的名字**：`strong`、`emphasis`、`strike`、`code`、`link`、`image`、`math`、`html`、`escape`、`entity`、`footnote-ref`、`fence`（代码块的围栏和 info）、`rule`。`syntax` 选中全部符号，`syntax.code` 只选中反引号。
- **它是所属构造的兄弟，不是子节点。**所以它继承构造外面的文字样式：`**` 不加粗，反引号不用等宽字体和代码底色。这和今天的外观一致。

`token` 的类是 MyGo 词法单元的种类：`keyword type function variable property constant number string escape regexp comment operator punctuation tag attribute meta heading emphasis strong link code inserted deleted invalid`。主题没有给颜色的种类，沿用 MyGo 代码编辑器的默认配色。

**伪元素**：编辑器画出来、但不是语法节点的东西。

| 伪元素 | 说明 |
|---|---|
| `item::marker` | 列表符号或编号 |
| `item::checkbox` | 任务复选框 |
| `code.block::info` | 代码块右上角的语言标签 |
| `footnote::label` | 脚注定义左侧的 `[^1]` |
| `::selection` | 选区 |
| `::search-text`、`::search-text:current` | 查找结果、当前选中的那一个 |
| `::placeholder` | 空文档里的占位文字 |

**伪类**

| 伪类 | 说明 |
|---|---|
| `:first-child`、`:last-child` | 在兄弟节点中的位置 |
| `:revealed` | 光标在这个构造里，它的语法正在显示。块（代码、公式、分隔线等）和行内构造（强调、链接、代码、公式、图片等）都有 |
| `:broken` | 图片没加载出来，或公式排不出来 |
| `:not(…)` | 参数是一个复合选择器 |

`:revealed` 如果改了字号这类影响排版的属性，光标移进移出时文字会跳动。这是主题作者的选择，引擎不拦（Typora 也是这样）。

### 3.4 Kind 与 Element

`Kind` 是 Go 侧的枚举，给写代码的人用：可以 `switch`，有类型检查。`Element` 是主题侧的字符串，给写 CSS 的人用。两者一一对应，都由 `syntax` 包定义，文档里列全。

## 4. 主题语言 `theme`

### 4.1 层叠层

一个主题是有序叠起来的几层样式表：

1. 引擎初始值（§4.4 属性表的"初始值"列）
2. `default.css`：内置，相当于浏览器自带的默认样式表
3. 主题文件：用户选的，可以有多个
4. 覆盖层：应用生成的，比如把"字号"设置写成 `document { font-size: 18px }`

规则：

- 同一层内，先比 `!important`，再比选择器特异性，最后比出现顺序。
- 不同层之间，后面的层胜出。
- `!important` 声明压过所有层里的普通声明；`!important` 之间也是后面的层胜出。
- 主题只需写它要改的东西，没写的沿用 `default.css`。

### 4.2 选择器

支持：

- 类型 `heading`、通配 `*`、类 `.h1`
- 属性 `[lang]`、`[lang="go"]`，运算符 `=` `~=` `^=` `$=` `*=`，末尾加 ` i` 不区分大小写
- 伪类和伪元素：见 §3.3
- 组合：后代（空格）、子（`>`）
- 选择器列表 `a, b`
- `:root` 等于 `document`

不支持的写法会报警告，并丢弃整条规则：兄弟组合 `+` `~`、`:nth-*`、`:hover` 等交互状态、`::before`/`::after`、命名空间。

### 4.3 At 规则与变量

- **`@media (prefers-color-scheme: dark | light) { … }`**：按编辑器**所在窗口**的主题判断（`ui.Theme.Dark`），不按操作系统。
- **`@font-face { font-family: "…"; src: url(…); font-weight: …; font-style: … }`**：路径相对主题文件；只支持 TTF、OTF、TTC（这是 `ui.RegisterFont` 能读的格式）。字体在主题生效时注册。
- **自定义属性**：`--x: …` 和 `var(--x, 回退值)`，按 CSS 规则继承。
- **编辑器注入的 `--ui-*` 变量**：窗口主题的颜色和字体，包括 `--ui-text`、`--ui-text-muted`、`--ui-background`、`--ui-surface`、`--ui-border`、`--ui-accent`、`--ui-accent-text`、`--ui-selection`、`--ui-danger`、`--ui-warning`、`--ui-success`、`--ui-font`、`--ui-font-size`、`--ui-radius`。`default.css` 从这些变量取色，所以默认外观会跟着应用的强调色和深浅色变。
- **图表变量**：`--diagram-muted`、`--diagram-line`、`--diagram-fill`、`--diagram-accent`，由图表渲染器读取（见 §5.4）。
- 其他 at 规则（`@import`、`@supports`、`@keyframes` 等）：报警告，忽略。

### 4.4 属性

#### 文字属性（可继承）

| 属性 | 适用于 | 初始值 | 说明 |
|---|---|---|---|
| `color` | 全部 | `var(--ui-text)` | |
| `font-family` | 全部 | `system-ui` | 逗号分隔的候选，取第一个本机已安装或已注册的。`system-ui`、`sans-serif` 指窗口主题的字体，`monospace` 指系统等宽字体 |
| `font-size` | 全部 | `16px` | `px`；`em` 和 `%` 相对父元素；`rem` 相对 `document` |
| `font-weight` | 全部 | `400` | 100–900、`normal`、`bold` |
| `font-style` | 全部 | `normal` | `normal`、`italic` |
| `font-feature-settings` | 全部 | `normal` | `"tnum"`、`"ss01" 1`、`"liga" 0` |
| `font-variant-numeric` | 全部 | `normal` | `tabular-nums`（等宽数字） |
| `letter-spacing` | 全部 | `normal` | 长度 |
| `line-height` | 块 | `1.75` | 倍数、长度或百分比，换算成倍数 |
| `text-align` | `paragraph` `heading` `cell` | `start` | `left` `center` `right`；单元格默认取 Markdown 里的对齐 |
| `caret-color` | 全部 | `auto` | 光标落在这个元素里时的颜色；`auto` 等于 `color` |
| `list-style-type` | `list` `item` | `auto` | `disc` `circle` `square` `decimal` `none`，或一个字符串。`auto`：无序列表按嵌套深度轮换 disc → circle → square，有序列表用 decimal |

#### 其他属性（不继承）

| 属性 | 适用于 | 初始值 | 说明 |
|---|---|---|---|
| `text-decoration-line`，以及简写 `text-decoration` | 行内元素和文字块 | `none` | `underline`、`line-through`，可以同时写；会向后代传播 |
| `text-decoration-style` | 同上 | `solid` | `solid`、`wavy` |
| `text-decoration-color`、`text-decoration-thickness` | 同上 | 文字色、自动 | |
| `vertical-align` | 行内元素 | `baseline` | `super`：上移父元素字号的 0.35 倍；`sub`：下移 0.2 倍；也可以写长度 |
| `opacity` | 全部 | `1` | 乘到元素及其后代的所有颜色上 |
| `background-color`，以及只接受颜色的简写 `background` | 行内元素、块、容器、伪元素 | `transparent` | 行内元素：画在文字背后。块：画满盒子。容器（引用、列表、列表项、脚注）：从第一个子块到最后一个子块的整片区域 |
| `border-radius` | 有背景或边框的元素、`image` | `0` | 只接受一个值 |
| `padding` 及四个方向 | 块、容器、`cell`、`document` | `0` | 容器的 `padding-left` 就是缩进 |
| `margin-top`、`margin-bottom`，以及简写 `margin` | 块、容器 | `0` | 简写只取上下两个值 |
| `border` 及各方向、`-width`、`-color`、`-style` | 块、容器、`table` | `none` | 线型只支持 `solid` 和 `none` |
| `max-width` | `document` | `820px` | 正文栏宽 |
| `height` | `rule`、`::checkbox` | | 分隔线盒子的高度；复选框的边长 |
| `width` | `::checkbox` | | 复选框的边长 |
| `margin-right` | `::marker`、`::checkbox` | `0` | 列表符号或复选框和正文之间的距离 |

#### 几个元素的特殊约定

- **`rule`**：线的粗细和颜色取自 `border-top`，盒子高度取自 `height`，线画在盒子的垂直中间。
- **`table`**：网格线的颜色和粗细取自 `table` 的 `border-color` 和 `border-width`，画成完整网格。表头底色用 `row.header { background-color }`。
- **`document`**：
  - `padding-top`：正文上方的留白。
  - `padding-left`、`padding-right`：正文栏两侧至少留多少空间。
  - `background-color`：编辑器的底色，默认透明，透出窗口的颜色。
  - 正文下方可以多滚动大约三分之一个视口，这是引擎行为，不由 CSS 控制。

#### 值与单位

- **长度**：`px`（即 MyGo 的 DIP）、`em`、`rem`、`%`、`0`。
- **颜色**：
  - `#rgb`、`#rgba`、`#rrggbb`、`#rrggbbaa`
  - `rgb()`、`rgba()`：逗号语法或空格语法，可写 `/ alpha`
  - `hsl()`、`hsla()`
  - 全部 CSS 命名颜色、`transparent`、`currentColor`
  - `color-mix(in srgb, A p%, B)`
- **通用关键字**：`inherit`、`initial`、`unset`。
- **函数**：`var()`、`calc()`、`min()`、`max()`、`clamp()`。长度在计算时统一换成 px，所以 `em` 和 `px` 可以混算。默认主题要靠它们还原一些值，比如行内代码的圆角今天是 `max(3px, 字号的 0.2 倍)`。

### 4.5 计算规则

- **`em` 和 `rem`**：`em` 在 `font-size` 上相对父元素的字号，在其他属性上相对元素自己的字号；`rem` 相对 `document` 的字号。
- **块之间的距离**：两个相邻块之间的距离，取交界处所有外边距的最大值。交界处的外边距包括：前一块的下外边距、前一块结束的那些容器的下外边距、后一块的上外边距、后一块开始的那些容器的上外边距。
  - 容器的内边距或边框会把内外两侧的外边距隔开，和 CSS 的外边距折叠规则一致。
  - 文档第一个块的上外边距、最后一个块的下外边距都不计。
- **行内元素的背景、装饰线、`vertical-align`**：作用于它所有后代文字，和浏览器的视觉效果一致。后代自己设了背景的，用后代自己的。例如 `<mark>` 里的 `**加粗**` 照样有高亮底色。
- **`syntax` 元素**：它是所属构造的兄弟，继承构造外面的样式（见 §3.3）。它不接收外层传播下来的背景和装饰线：今天的语法符号就没有底色、下划线和删除线，哪怕它在 `<mark>` 或链接里。基线偏移照常传播，所以 `<sup>` 里的符号仍然跟着上移。
- **`opacity`**：乘到元素自身和所有后代的文字色、背景色、边框色上。
- **未聚焦时的选区**：编辑器失去焦点时，`::selection` 的颜色由引擎再乘 0.6，和今天一致。
- **留在引擎里、不进 CSS 的绘制细节**：
  - 容器左边框（引用竖条）向内缩 1px，上下各多出 2px，圆角取宽度的一半；
  - 列表符号右对齐到缩进栏；
  - 复选框和对勾的形状，复选框与首行基线对齐；
  - 代码块语言标签贴在右上角；
  - 图片按比例缩小，不超过栏宽；
  - 选区和搜索高亮的圆角。

### 4.6 诊断

`Parse` 不会失败。它像浏览器一样，遇到坏掉的部分就跳过，从下一个能读懂的地方继续，并返回一个诊断列表：

```go
type Diagnostic struct {
	Severity  Severity // Error 或 Warning
	Line, Col int      // 从 1 开始
	Message   string   // 例如：unknown property "font-szie" (did you mean "font-size"?)
}
```

- **Error**：语法错误，比如块、字符串或括号没有闭合。从下一条规则继续读。
- **Warning**：
  - 不认识的元素、类、伪类或属性。按编辑距离给出"你是不是想写……"的建议。
  - 类和元素对不上，比如 `heading.h7`、`code.ordered`。v1 的词汇表是固定的，所以能做这个校验。
  - 值不合法。
  - 属性不适用于该元素。
  - 用了不支持的特性。

### 4.7 Go API

```go
package theme

func Parse(name, css string) (*Sheet, []Diagnostic)
func ParseFile(path string) (*Sheet, []Diagnostic, error) // url() 相对文件所在目录
func Default() *Sheet                                      // 内置的 default.css

func New(sheets ...*Sheet) *Theme     // 叠在 Default() 之上
func (t *Theme) With(s *Sheet) *Theme // 再叠一层，比如覆盖层
func (t *Theme) FontFaces() []FontFace // 需要注册的字体文件，由编辑器调用 ui.RegisterFont

// 下面是编辑器用的接口
type Env struct {
	Dark bool
	Vars map[string]string // --ui-*
}
func (t *Theme) Resolver(env Env) *Resolver
func (r *Resolver) Root() *Node            // document
func (n *Node) Child(el Element) *Node     // 同一个元素只建一次（前缀树）
func (n *Node) Style() *Computed           // 第一次用到时计算，之后缓存
func (n *Node) Pseudo(name string, states States) *Computed
// Computed 里所有值都已换算成 DIP 和颜色；font-family 保留候选列表，由编辑器挑选。
```

样式路径的前缀树（§5.1）就在 `theme` 包里，编辑器只持有节点。这样缓存和失效逻辑可以脱离窗口单独测试。

CSS 的分词器自己写（CSS Syntax Level 3 的子集），不引入第三方依赖。这样能精确掌握行列号和错误恢复。

## 5. 编辑器接入

### 5.1 样式路径

**今天的做法**：编辑器用 `istyle`（一组布尔值）沿行内树往下传，最后由 `look.span()` 翻译成 `ui.Span`。块级的字号、间距、颜色则散在 `layout.go`、`paint.go` 和常量里。

**之后的做法**：块、行内构造、伪元素，各自是路径上的一个节点，例如：

```
document › quote › list.bullet.tight › item.task.checked › paragraph › strong › code.inline
```

- **缓存**：路径节点组织成一棵前缀树（`theme.Node`），相同的路径只建一次。每个节点的计算样式在第一次用到时算出并缓存。
- **失效**：主题、深浅色或 `--ui-*` 变量任何一项变了，编辑器换一个新的 `Resolver`，旧树整棵丢弃。
- **性能**：一个文档里不同的路径通常只有几十到几百条，打字时只会碰到已有的路径，所以不会因为算样式而变慢。
- **代码层面的改动**：
  - builder 里的 `st istyle` 换成 `st *theme.Node`；
  - `b.span(st)` 换成 `st.span()`；
  - 语法符号从构造的父节点派生出 `syntax.<构造>` 节点；
  - leaf 的视图缓存键里，`lookKey` 换成"样式节点 + 主题版本"。

### 5.2 替换清单

| 今天写死的地方 | 之后由什么决定 |
|---|---|
| `look` 的颜色字段、`setLook` 的深浅色分支 | `default.css` 里 `:root` 的变量，加 `@media` |
| `headingSize`、`headingWeight`、标题行高 1.35 | `heading.hN { font-size; font-weight; line-height }` |
| `margins()` 里各类块的间距 | 各元素的 `margin-*` |
| 紧凑列表的 0.2em 间距 | `list.tight > item > * { margin }`，加上 `list` 自己的外边距。对照测试说了算 |
| `quoteIndent`、`itemIndent` | `quote`、`item`、`footnote` 的 `padding-left` |
| `codePadX/Y`、代码字号 0.875、代码行高 1.6 | `code.block { padding; font-size; line-height }` |
| `cellPadX/Y`、表格线颜色（`paint.go` 里写死） | `cell { padding }`、`table { border-color }` |
| 列表符号 •◦▪、编号用等宽数字、符号颜色 | `list-style-type`、`item::marker` |
| 复选框颜色（`paint.go` 里写死的白色对勾） | `item::checkbox`、`item.checked::checkbox` |
| 引用竖条、引用里的文字变灰 | `quote { border-left; color }` |
| 已勾选任务的首行变灰并加删除线 | `item.checked > paragraph:first-child { … }` |
| 表头加粗、表头底色 | `cell.header { font-weight }`、`row.header { background-color }` |
| 行内代码、kbd、mark、sup、sub、链接、脚注引用 | 各自元素的规则 |
| 链接里的代码沿用链接颜色 | `link code.inline { color }` |
| 语法符号的颜色 | `syntax { color }` |
| 选区颜色 | `::selection`（失去焦点时的 0.6 倍仍由引擎处理） |
| 搜索高亮（写死的黄色、橙色） | `::search-text`、`::search-text:current` |
| 光标颜色 | `caret-color` |
| 占位文字 | `::placeholder` |
| 图片占位框，以及写死的中文"🖼 图片" | `image:broken { … }`；文字改成 🖼 加 alt，没有 alt 就用文件名 |
| 公式颜色只区分深浅色（`dark bool`） | 渲染器拿 `math` 元素的计算颜色 |
| Mermaid 直接用窗口主题的颜色 | 渲染器拿代码块的计算样式和 `--diagram-*` 变量 |
| 代码高亮配色（MyGo `editor.DefaultTheme`） | `token.<种类>`；没设的沿用 MyGo 默认 |
| `Options` 里的 FontSize、LineHeight、Font、MonoFont、MaxWidth、PadX | 移除，改由 CSS 决定；应用想调整就生成一层覆盖样式 |
| 正文顶部留白 2.2em | `document { padding-top }` |

### 5.3 Options 与 API 变化

```go
type Options struct {
	Theme     *theme.Theme      // nil：只用 default.css
	Math      MathRenderer      // nil：公式以源码显示
	Diagrams  []DiagramRenderer // 空：图表以代码显示
	LoadImage func(src string, onLoad func()) *ui.Bitmap // nil：内置加载器（本地文件、data:、http）

	// 以下与今天相同
	BaseDir, Placeholder, Label string
	Invalidate, OnChange        func()
	OnLink                      func(url string)
	SaveImage                   func(data []byte, ext string) (string, error)
	OnDropFiles                 func(paths []string)
	ReadClipboard               func() transfer.Data
	WriteClipboard              func(transfer.Data)
}

func (e *Editor) Syntax() *syntax.Tree                // 当前源码的语法树快照
func (e *Editor) StylePath(off int) []theme.Element // 某个位置实际匹配的路径（含状态），给检查器用
```

- **移除**：`Code`、`FontSize`、`LineHeight`、`Font`、`MonoFont`、`MaxWidth`、`PadX`。
- **`Label` 的默认值**：从中文"文档"改成 "Document"。

### 5.4 渲染器

```go
// MathRenderer 排版 TeX。
type MathRenderer interface {
	Math(tex string, display bool, size float32, color ui.Color) (img *ui.Bitmap, baseline float32, ok bool)
}

// DiagramRenderer 把它认识的语言的代码块画成图。
type DiagramRenderer interface {
	Diagram(lang, code string) (Diagram, bool)
}

type Diagram interface {
	Height(width float32) float32
	Draw(p *ui.Painter, r ui.Rect, s DiagramStyle)
}

type DiagramStyle struct {
	Text, Muted, Line, Fill, Accent ui.Color
	FontSize                        float32
}
```

- `render/math.New()` 和 `render/mermaid.New()` 从 `internal/mdoc` 拆出来。
- 公式的颜色从此跟着主题走；缓存键从 `(表达式, 字号, dark)` 改成 `(表达式, 字号, 颜色)`。
- `DiagramStyle` 的来源：`Text` 取代码块计算样式里的 `color`，其余四个颜色取 `--diagram-*` 变量。
- 根包不再依赖 go-latex：不用公式的应用，编译产物里没有它。

## 6. Playground `cmd/playground`

做法和 MyGo 文档里的实时预览一样：同一个 Go 原生 UI 程序，桌面上用 `go run ./cmd/playground` 运行，浏览器里是编译成 WebAssembly 的静态页面。可行性已经验证：livemark 能编译到 `js/wasm`，一个最小应用 22 MB，gzip 后 5.8 MB。

界面：

- **左栏：主题 CSS 编辑器**（MyGo `ui/editor`，CSS 高亮），上方一排预设主题（默认、纸张）。停止输入约 150ms 后重新解析，立刻生效。
- **右栏：livemark 编辑器**，打开一篇覆盖词汇表里每一种元素的示例文档。
- **底栏：诊断**。列出主题的错误和警告，点一条就跳到 CSS 编辑器里对应的行列。
- **状态栏：节点路径检查器**。显示光标处的样式路径（如 `document › quote › paragraph › strong:revealed`），一键把它作为选择器插入 CSS 编辑器。主题作者不用猜该写什么。
- **语法树面板**（可开关）：展示 `syntax.Tree` 的节点和区间，顺便演示公开的语法树 API。
- **顶栏**：深浅色切换。
- **桌面版独有**：打开、保存主题文件；文件在别处被改动时热重载。

示例主题"纸张"：衬线正文、暖色、标题下划线、斑马纹表格。它用来证明 CSS 子集的表达力，也给主题作者当范本。

网页版：

- 构建脚本用 `GOOS=js GOARCH=wasm` 编译，配上 Go 自带的 `wasm_exec.js` 和一个加载页（加载时显示转圈），产物是静态文件。
- GitHub Actions 发布到 GitHub Pages。
- 浏览器里读不到本地文件，所以示例文档的图片用内嵌的 data: URL。浏览器也不告诉网页本机有哪些字体，`font-family` 在浏览器里如何回退，要在实现时按 MyGo 的 js 后端确认。

## 7. 测试

1. **现有测试**：根包 47 个、`internal/mdoc` 11 个，全程保持通过。
2. **外观对照基线**：在任何重构之前先建好。
   - 把 `zz_dump_test.go` 的思路做成正式测试。
   - 输入：一组覆盖全部元素的文档 × 深浅色 × 若干光标位置（语法显示和隐藏两种情况）。
   - 记录三样东西：
     - 每个 leaf 的盒子（x、y、w、h）；
     - 每段显示文字的样式（颜色、背景、字号、字重、斜体、装饰线、字体、圆角、基线偏移）；
     - 离屏截图。
   - 全部存成 golden 文件。
   - 重构后用 `default.css` 再跑一遍：结构化快照必须完全相同，截图必须逐像素相同。
   - 有两处**有意的差异**，单独列出，对照时豁免：
     - **深色模式下公式的边缘像素。**今天浅色用纯黑墨，深色把整张位图反色。反色时直接对预乘 alpha 的像素取反，抗锯齿边缘会得到不合法的颜色。新渲染器按颜色正确着色，`default.css` 仍然给公式用纯黑和纯白，所以只有边缘像素不同。
     - **图片加载失败时的占位文字。**从写死的"🖼 图片"改成 🖼 加 alt 或文件名。
3. **语法层**：
   - 用 CommonMark 规范用例和自己写的用例，生成节点树 golden（类型、区间、语法符号）。
   - 一致性测试：对语料里的每个位置，`syntax.PathAt` 得到的元素路径必须等于编辑器的样式路径。
4. **主题引擎**：纯 Go 单测，不需要窗口。覆盖：
   - 分词，错误恢复，诊断的行列号；
   - 选择器匹配和特异性，层叠层和 `!important`，继承；
   - 单位换算、`var()`、颜色解析；
   - `@media`、`@font-face`。
5. **性能**：
   - 把 `perf_test.go`（今天只打印耗时）改成用固定语料的 Go benchmark：打字、移动光标、全文重排、切换主题。
   - 重构前后用 benchstat 对比，退化不超过 10%。

## 8. 里程碑

| 里程碑 | 内容 | 完成标准 |
|---|---|---|
| M0 ✅ | fork 基线 | `fd86c85`，构建和测试通过 |
| M0.5 ✅ | 同步 Muse 完成后的 CJK 强调工作 | 按 Muse `1a86fd4` 移植 `CJKFriendly`，测试通过；该提交里的高亮、块级 HTML 等改动不属 v1，未移植 |
| M1 ✅ | 外观对照基线 | `testdata/appearance` 的 golden 在 M0.5 后的代码上稳定通过 |
| M2 ✅ | `syntax` 包 | 公开 API 和词汇表文档完成；解析准备搬进 `internal/parse`；一致性测试通过 |
| M3 ✅ | `theme` 包 | CSS 子集的解析、层叠、诊断完成，单测通过；`default.css` 写完，解析时没有警告 |
| M4 ✅ | 编辑器改用主题 | `look`、`istyle` 和写死的常量全部移除；对照基线一致；性能达标 |
| M5 ✅ | 拆分渲染器 | `render/math`、`render/mermaid` 独立；公式和图表颜色跟随主题；根包不依赖 go-latex |
| M6 ✅ | Playground、示例主题、文档 | 桌面版的 CSS 编辑、诊断、检查器、语法树面板、热重载可用；README 和主题作者文档（词汇表、属性参考）完成 |
| M7 ✅ | 网页版 playground | `js/wasm` 构建脚本和加载页完成；GitHub Actions 发布到 GitHub Pages |

**顺序约束**：

- M0.5 已先于 M1 完成：golden 录在包含 CJK 最终版的代码上。
- M1 已先于 M2 和 M4 完成：它是这两步的安全网。
- M3 是独立的纯 Go 包，不碰编辑器代码，所以最早完成，不用等 CJK 那边。

## 9. 风险

- **CJK 工作还在进行中。**基线里的 `cjk.go` 是半成品，所以 M0.5 必须排在 M1 之前（见 §8）。
- **"看着像 CSS 但又不完全是"。**作者会去试 `display: flex`、`::before`。应对：诊断明确提示"不支持"，文档列全支持的子集。
- **外观对照的精确度。**少数绘制细节（引用竖条内缩、复选框对勾的形状）留在引擎里，不进 CSS，由对照测试保证它们不变。紧凑列表的间距要改用容器外边距表达，`default.css` 以对照测试结果为准来调。
- **字体注册是整个进程共享的。**`ui.RegisterFont` 没有注销接口。热重载时按内容哈希去重；同名但内容不同的字体，以最后一次注册的为准。
- **MyGo 还在 0.x。**固定在 v0.7.3，升级单独做。
- **与 Muse 分叉。**这一点已经接受。基线提交记录了来源，以后移植修复时按文件对比。

## 10. 已定事项

评审时（2026-10-30）定下来的：

1. 示例主题用"纸张"风格。
2. 公开文档和 Muse 一样：README 以英文为主，另附中文版。
3. 远程图片和今天一样默认加载，应用可以通过 `LoadImage` 换掉加载器、关闭联网。
4. 加入 playground（§6），像 MyGo 的文档预览一样能在网页上运行。
