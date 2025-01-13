<template>
  <div class="w-full rounded-lg bg-lightest my-4">
    <div class="p-3">
      <p class="text-lg font-bold">Who to follow</p>
    </div>
    <button v-for="suggestion in followSuggestions" :key="suggestion.id" class="w-full flex hover:bg-lighter p-3 border-t border-lighter">
      <img :src="suggestion.avatar" class="w-12 h-12 rounded-full" />
      <div class="hidden lg:block ml-4">
        <p class="text-left text-sm font-bold leading-tight">{{ suggestion.name }}</p>
        <p class="text-left text-sm leading-tight text-dark">{{ suggestion.username }}</p>
      </div>
      <button class="ml-auto text-sm text-blue font-bold px-4 py-1 rounded-full border border-blue m-2">
        Follow
      </button>
    </button>
    <button class="p-3 w-full hover:bg-lighter text-left text-blue border-t border-lighter">
      Show More
    </button>
  </div>
</template>

<script>
export default {
  name: 'WhoToFollow',
  data() {
    return {
      followSuggestions: [
        {avatar: 'https://i.imgur.com/wHoKLev.jpg', name: 'Hanna', username: '@bestofallhans', id: 'bestofallhans'}
      ],
    };
  },
};
</script>