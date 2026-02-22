module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  corePlugins: {
    preflight: false
  },
  theme: {
    screens: {
      sm: '640px',
      md: '768px',
      lg: '1024px',
      xl: '1440px',
      '2xl': '1536px'
    },
    spacing: {
      0: '0px',
      4: '4px',
      8: '8px',
      12: '12px',
      16: '16px',
      20: '20px',
      24: '24px'
    },
    extend: {
      width: {
        100: '100px',
        140: '140px',
        150: '150px',
        200: '200px',
        260: '260px',
        300: '300px',
        320: '320px'
      },
      borderRadius: {
        4: '4px',
        6: '6px'
      },
      boxShadow: {
        card: '0 2px 12px 0 rgba(0, 0, 0, 0.1)',
        'card-hover': '0 4px 16px 0 rgba(0, 0, 0, 0.15)'
      },
      fontSize: {
        12: '12px',
        13: '13px',
        14: '14px',
        15: '15px',
        16: '16px',
        20: '20px',
        24: '24px',
        28: '28px'
      }
    }
  }
}
