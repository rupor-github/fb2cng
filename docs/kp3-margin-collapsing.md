# KP3 Margin Collapsing Algorithm

Derived from: `EpubToKFXConverter-4.0.jar` (Kindle Previewer 3) version 3.101.0

## Source Files Reference

All source files are located in `/home/rupor/amazon/src/` (decompiled KP3 JAR).

### Key Files

| File                                                                | Purpose                                                         |
| ------------------------------------------------------------------- | --------------------------------------------------------------- |
| `com/amazon/yj/v/c/d.java`                                          | Main margin collapser entry point - orchestrates the 3 phases   |
| `com/amazon/yj/v/c/a.java`                                          | Phase 1: Root/container margin consolidation                    |
| `com/amazon/yj/v/c/b.java`                                          | Phase 2: Parent-child margin collapsing                         |
| `com/amazon/yj/v/c/c.java`                                          | Phase 3: Adjacent sibling collapsing                            |
| `com/amazon/yj/v/h/f.java`                                          | Collapse prevention flags (border, padding, position, etc.)     |
| `com/amazon/yj/v/i/k.java`                                          | Utility functions including the collapse calculation dispatcher |
| `com/amazon/yj/style/merger/rules/YJOverrideMaximumRuleMerger.java` | Rule merger for collapse calculations                           |
| `com/amazon/yj/style/merger/e/f.java`                               | **Core collapse algorithm** (lines 231-274)                     |
| `com/amazon/yj/d/c.java`                                            | Constants including `doMarginCollapse` (line 51)                |
| `com/amazon/yj/i/b/b.java`                                          | Fragmentor - margin stripping for 64K splits (lines 341-352)    |
| `com/amazon/yj/i/b/a/a.java`                                        | Property name constants (`margin_top`, `margin_bottom`, etc.)   |
| `com/amazon/adapter/h/b/b/a.java`                                   | Adapter that can set `doMarginCollapse=false` (line 163)        |

---

## Algorithm Overview

KP3's margin collapser runs in the `com/amazon/yj/v/c/d.java` class. The main entry point is method `a()` (lines 20-28):

```java
public void a() {
   try {
      this.b();  // Phase 1: Root consolidation (class a)
      this.d();  // Phase 2: Parent-child collapsing (class b)
      this.c();  // Phase 3: Sibling collapsing (class c)
   } catch (Exception var2) {
      e.d(com.amazon.i.b.YJ_MARGIN_COLLAPSER_GENERAL_INFO, "Error while collapsing margins." + var2.getMessage());
   }
}
```

---

## Phase 1: Root/Container Margin Consolidation

**File:** `com/amazon/yj/v/c/a.java`

This phase consolidates MARGIN_BLOCK_START and MARGIN_BLOCK_END on the root container itself. If the container is collapsible (`k.c()` returns true), it:

1. Gets both margin values from the container's style
2. Collapses them using `k.a()`
3. Removes MARGIN_BLOCK_END
4. Sets MARGIN_BLOCK_START to the collapsed value

---

## Phase 2: Parent-Child Margin Collapsing

**File:** `com/amazon/yj/v/c/b.java`

This phase handles margin collapsing between a container and its first/last children.

### Entry Conditions (method `d()`, line 122-123)

```java
private boolean d() {
   return this.a.k() != com.amazon.B.d.e.h.e.CONTENT_LIST ? false : this.a.o() != 0;
}
```

Only processes containers of type CONTENT_LIST with at least one child.

### Collapse Prevention Check (method `a()`, line 126-127)

```java
private boolean a(com.amazon.yj.v.h.f var1) {
   return !var1.f() && !var1.a() && !var1.b();
}
```

Collapsing is prevented if:

- `f()` - Container is table-related or has `doMarginCollapse=false`
- `a()` - Container has POSITION or FLOAT
- `b()` - Container has CLIP=true

### First Child Collapsing (method `c()`, lines 78-119)

```java
// Get first child (index 0)
f var1 = this.a.m().get(0);

// Check if first child allows collapsing AND parent doesn't block top collapse
com.amazon.yj.v.h.f var3 = new com.amazon.yj.v.h.f(var1, this.d, this.e);
com.amazon.yj.v.h.f var4 = new com.amazon.yj.v.h.f(this.a, this.d, this.e);
if (this.a(var3) && !var4.c()) {  // !c() means no border/padding on top
   // Collapse first child's margin-top with parent's margin-top
   // Result goes on PARENT, child's margin-top is REMOVED
   var10.a().a(var5);        // Remove child's margin-top
   var11.a().a(var5, var9);  // Set parent's margin-top to collapsed value
}
```

**Key behavior:**

- First child's margin-top collapses with parent's margin-top
- Collapsed value goes to PARENT
- Child's margin-top is REMOVED
- Blocked by parent's border-top or padding-top

### Last Child Collapsing (method `b()`, lines 38-76)

```java
// Get last child (index n-1)
f var2 = this.a.m().get(var1 - 1);

// Check conditions
com.amazon.yj.v.h.f var5 = new com.amazon.yj.v.h.f(var2, this.d, this.e);
com.amazon.yj.v.h.f var6 = new com.amazon.yj.v.h.f(this.a, this.d, this.e);
if (this.a(var5) && !var6.d() && !var6.e()) {  // !d() no border/padding bottom, !e() no height
   // Collapse last child's margin-bottom with parent's margin-bottom
   // Child's margin-bottom is REMOVED
   // Parent gets the collapsed value
   var11.a().a(var7);        // Remove child's margin-bottom
   var12.a().a(var7, var10); // Set parent's margin-bottom to collapsed value
}
```

**Key behavior:**

- Last child's margin-bottom collapses with parent's margin-bottom
- Collapsed value goes to PARENT
- Child's margin-bottom is REMOVED
- Blocked by parent's border-bottom, padding-bottom, height, or min-height

---

## Phase 3: Adjacent Sibling Collapsing

**File:** `com/amazon/yj/v/c/c.java`

This phase collapses margins between adjacent sibling elements.

### Main Logic (method `a()`, lines 30-61)

```java
if (this.a(this.a, this.b)) {  // Both siblings allow collapsing
   // Get margin values
   com.amazon.B.d.f.e var7 = this.a(var1, var5, var6, var6, var3);  // Predecessor's mb
   com.amazon.B.d.f.e var8 = this.a(var2, var5, var6, var5, var4);  // Successor's mt

   if (var8 != null && var7 != null) {
      // Collapse the margins
      com.amazon.B.d.f.e var9 = k.a(var5, var7, var8, this.d, this.e, true, ...);

      // REMOVE predecessor's margin-bottom
      if (var3) {
         this.a(var10);  // Strips both margins if empty node
      } else {
         var10.a().a(var6);  // Remove margin-bottom only
      }

      // SET successor's margin-top to collapsed value
      if (var4) {
         this.a(var11);
      }
      var11.a().a(var5, var9);  // Set margin-top to collapsed value
   }
}
```

**Key behavior:**

- Predecessor's margin-bottom is REMOVED
- Successor's margin-top is SET to the collapsed value
- Both elements must pass the collapse prevention check

### Sibling Collapse Prevention (method `a()`, lines 97-105)

```java
private boolean a(f var1, f var2) {
   com.amazon.yj.v.h.f var3 = new com.amazon.yj.v.h.f(var1, this.f, this.g);
   if (!var3.f() && !var3.a() && !var3.b()) {
      var3 = new com.amazon.yj.v.h.f(var2, this.f, this.g);
      return !var3.f() && !var3.a() && !var3.b();
   } else {
      return false;
   }
}
```

---

## Core Collapse Calculation

**File:** `com/amazon/yj/style/merger/e/f.java`, lines 231-274

This is the actual mathematical calculation for collapsing two margin values:

```java
public static double a(
   com.amazon.yj.style.merger.d.d var0,  // First margin
   com.amazon.yj.style.merger.d.d var1,  // Second margin
   com.amazon.yj.style.merger.d.b var2,
   com.amazon.yj.style.merger.d.b var3,
   boolean var4,                          // isSiblingCollapse
   boolean var5,                          // special handling flag
   s var6,                                // property name
   com.amazon.B.d.e.b.f var7,
   com.amazon.B.d.b var8,
   com.amazon.f.b.a.a.b var9
) throws com.amazon.s.c {
   // ... setup code ...

   f = var0.a();  // First margin value
   g = var1.a();  // Second margin value
   double var10 = 0.0;

   if (f >= 0.0 && g >= 0.0) {
      // BOTH POSITIVE: use maximum
      var10 = Math.max(f, g);
   } else if (f <= 0.0 && g <= 0.0) {
      // BOTH NEGATIVE: use minimum (most negative)
      var10 = Math.min(f, g);
   } else if (f < 0.0 || g < 0.0) {
      // MIXED SIGNS
      if (var5) {
         // Special sibling handling (complex algorithm in method a())
         var10 = a();
      } else {
         // Parent-child: simple addition
         var10 = f + g;
      }
   }

   return var10;
}
```

### Collapse Rules Summary

| Margin 1             | Margin 2 | Result                               |
| -------------------- | -------- | ------------------------------------ |
| Positive             | Positive | `max(m1, m2)`                        |
| Negative             | Negative | `min(m1, m2)` (most negative)        |
| Mixed (sibling)      |          | Complex algorithm - see `a()` method |
| Mixed (parent-child) |          | `m1 + m2`                            |

---

## Collapse Prevention Flags

**File:** `com/amazon/yj/v/h/f.java`

This class determines when margin collapsing should be prevented based on CSS properties.

### Table-Related Containers (lines 21-28)

```java
private static final Set<com.amazon.B.d.e.h.f> k = Collections.unmodifiableSet(new HashSet<com.amazon.B.d.e.h.f>() {
   {
      this.add(com.amazon.B.d.e.h.f.TABLE_BODY);
      this.add(com.amazon.B.d.e.h.f.TABLE_ROW);
      this.add(com.amazon.B.d.e.h.f.TABLE_FOOTER);
      this.add(com.amazon.B.d.e.h.f.TABLE_HEADER);
   }
});
```

### The `doMarginCollapse` Attribute (lines 44-57)

```java
private boolean a(com.amazon.B.d.e.b.f var1) {
   // Table-related containers don't collapse
   if (k.contains(var1.t())) {
      return true;
   }
   // Parent is TABLE_ROW - don't collapse
   com.amazon.B.d.e.b.f var2 = var1.l();
   if (var2 != null && com.amazon.B.d.e.h.f.TABLE_ROW.equals(var2.t())) {
      return true;
   }
   // Check explicit doMarginCollapse attribute
   com.amazon.B.d.f.a var3 = var1.g();
   com.amazon.B.d.f.e var4 = var3.a("doMarginCollapse");
   return var4 != null && !var4.d();  // If doMarginCollapse=false, prevent collapse
}
```

### Property-Based Collapse Prevention (method `g()`, lines 59-98)

```java
private void g() {
   if (this.a != null) {
      if (this.a.a() != null) {
         for (s var3 : this.a.a().b()) {
            // POSITION or FLOAT: prevent top collapse
            if (var3.equals(s.POSITION) || var3.equals(s.FLOAT)) {
               this.b = true;  // hasPositionOrFloat
            }

            // BORDER_WEIGHT, PADDING, BORDER_STYLE: prevent both top and bottom
            if (var3.equals(s.BORDER_WEIGHT) || var3.equals(s.PADDING) || var3.equals(s.BORDER_STYLE)) {
               this.c = true;  // blockStartBlocked
               this.d = true;  // blockEndBlocked
            }

            // Block-start specific: BORDER_WEIGHT_BLOCK_START, PADDING_BLOCK_START, BORDER_STYLE_BLOCK_START
            if (var3.equals(BORDER_WEIGHT_BLOCK_START) || var3.equals(PADDING_BLOCK_START) || var3.equals(BORDER_STYLE_BLOCK_START)) {
               this.c = true;  // blockStartBlocked
            }

            // Block-end specific: BORDER_WEIGHT_BLOCK_END, PADDING_BLOCK_END, BORDER_STYLE_BLOCK_END
            if (var3.equals(BORDER_WEIGHT_BLOCK_END) || var3.equals(PADDING_BLOCK_END) || var3.equals(BORDER_STYLE_BLOCK_END)) {
               this.d = true;  // blockEndBlocked
            }

            // HEIGHT or MIN_HEIGHT: prevent bottom collapse
            if (var3.equals(BLOCK_SIZE) || var3.equals(BLOCK_MIN_SIZE)) {
               this.e = true;  // hasBlockSize
            }

            // CLIP=true: prevent collapse
            if (var3.equals(s.CLIP)) {
               com.amazon.B.d.f.e var4 = this.a.a().c(var3);
               if (var4.d()) {
                  this.f = true;  // hasClip
               }
            }
         }
      }
   }
}
```

### Flag Methods Summary

| Method | Field | Prevents        | Triggered By                                               |
| -----: | ----- | --------------- | ---------------------------------------------------------- |
| `f()`  | `g`   | All collapse    | Table containers, `doMarginCollapse=false`                 |
| `a()`  | `b`   | Top collapse    | `position`, `float`                                        |
| `b()`  | `f`   | All collapse    | `clip: true`                                               |
| `c()`  | `c`   | Top collapse    | `border-top`, `padding-top`, `border-style` (top)          |
| `d()`  | `d`   | Bottom collapse | `border-bottom`, `padding-bottom`, `border-style` (bottom) |
| `e()`  | `e`   | Bottom collapse | `height`, `min-height`                                     |

---

## The `doMarginCollapse` Attribute

**File:** `com/amazon/yj/d/c.java`, line 51

```java
public static final String V = "doMarginCollapse";
```

**File:** `com/amazon/adapter/h/b/b/a.java`, line 163

```java
com.amazon.adapter.h.j var2 = new com.amazon.adapter.h.j("doMarginCollapse", "false", t.VALUE_TYPE_BOOL);
```

The adapter sets `doMarginCollapse=false` when (lines 168-169):

```java
private boolean c(com.amazon.adapter.i.b.i var1) throws com.amazon.s.c {
   return this.h(var1) || com.amazon.adapter.i.b.k.G(var1) || com.amazon.adapter.i.b.k.ad(var1);
}
```

Where `h()` checks for `display: inline-block` (lines 260-263):

```java
private boolean h(com.amazon.adapter.i.b.i var1) throws com.amazon.s.c {
   Map var2 = var1.g();
   String var3 = (String)var2.get("display");
   return "inline-block".equals(var3);
}
```

---

## Fragment Margin Stripping (64K Splits)

**File:** `com/amazon/yj/i/b/b.java`, lines 341-352

When content exceeds 64K and must be split into fragments, margins are stripped based on fragment position:

```java
// First fragment: removes bottom margin/padding
if (isFirstFragment) {
   // Remove MARGIN_BLOCK_END, PADDING_BLOCK_END
}

// Last fragment: removes top margin/padding and text-indent
if (isLastFragment) {
   // Remove MARGIN_BLOCK_START, PADDING_BLOCK_START, TEXT_INDENT
}

// Middle fragments: removes both
if (isMiddleFragment) {
   // Remove all vertical margins and padding
}
```

**Note:** This is for 64K text splitting fragmentation, NOT for container margin collapsing.

---

## Property Constants

**File:** `com/amazon/yj/v/i/i.java`

```java
public static final String a = "margin_top";
public static final String b = "margin_bottom";
```

**File:** `com/amazon/yj/i/b/a/a.java`

```java
public static final String a = "margin_top";
public static final String b = "margin_bottom";
```

**File:** `com/amazon/yj/d/a.java`

```java
public static final String h = "margin-left";
public static final String i = "margin-right";
public static final String k = "margin-top";
public static final String l = "margin-bottom";
```

**File:** `com/amazon/yj/d/a/f.java` (Enum)

```java
MARGIN_TOP("margin_top"),
MARGIN_BOTTOM("margin_bottom"),
MARGIN_RIGHT("margin_right"),
MARGIN_LEFT("margin_left"),
```

