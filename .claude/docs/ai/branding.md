# Applied Intuition Branding

Use the official color palette for Applied-branded apps.

## CSS Variables

```css
:root {
    /* Primary */
    --applied-primary: #0F6AF2;

    /* Semantic */
    --applied-red: #EB3737;
    --applied-green: #389F3D;
    --applied-purple: #8B4AF5;

    /* Grays */
    --applied-gray-900: #000000;
    --applied-gray-700: #5C637A;
    --applied-gray-500: #9A9EAD;
    --applied-gray-300: #D8D9DF;
    --applied-gray-100: #F4F4F6;

    /* Backgrounds */
    --applied-bg-dark: #1a1a2e;
    --applied-bg-light: #ffffff;
}
```

## Tailwind Config

```javascript
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        applied: {
          primary: '#0F6AF2',
          red: '#EB3737',
          green: '#389F3D',
          purple: '#8B4AF5',
          gray: {
            100: '#F4F4F6',
            300: '#D8D9DF',
            500: '#9A9EAD',
            700: '#5C637A',
            900: '#000000',
          },
        },
      },
    },
  },
}
```

## Usage Examples

```jsx
// Primary button
<button className="bg-applied-primary text-white hover:bg-blue-700">
  Submit
</button>

// Error message
<p className="text-applied-red">Something went wrong</p>

// Success message
<p className="text-applied-green">Saved successfully</p>

// Muted text
<span className="text-applied-gray-500">Last updated 2 hours ago</span>
```

## Logo

The Applied logo is available at:
- Light background: Use dark logo
- Dark background: Use white logo

Request logo assets from the design team if needed.

